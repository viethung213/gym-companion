"""Coaching Context Agent — ADK Sequential Workflow (Python Prototype).

This is the DESIGN prototype incorporating the SkillToolset for progressive disclosure
of static business rules alongside dynamic function tools and ADK Guardrail Callbacks.

Architecture:
  @node fetch_user_context  → inject mandatory data (profile, sessions, roadmap, time)
  generator_agent (tools)   → dynamic reasoning: search_exercises, get_exercise_pr
                            → skill_toolset: coaching-roadmap-rules (rules, json-spec L3)
                            → before_model_callback: validate_input_safety (Guardrail 1)
                            → before_tool_callback: validate_tool_execution (Guardrail 2)
  @node parse_to_schema     → text → GeneratedPlan struct
  evaluator_agent           → input_schema + output_schema (FlowInitiate4Week only)
"""

from __future__ import annotations

import json
from datetime import datetime, timedelta, timezone

from google.adk import Agent, Context, Skill, SkillToolset, Resources, Frontmatter
from google.adk.workflow import node
from pydantic import BaseModel, Field

# ─── Config ───────────────────────────────────────────────────────────────────

_MODEL = "gemini-2.5-flash"
_APP_NAME = "coaching-context-agent"

# Flow constants — mirrors Go agent.FlowType
FLOW_INITIATE_4_WEEK = "INITIATE_4_WEEK"
FLOW_REGENERATE = "REGENERATE_PENDING"
FLOW_ADAPTIVE_CYCLE = "ADAPTIVE_CYCLE"
FLOW_SIGNAL_HANDLER = "SIGNAL_HANDLER"
FLOW_POST_INJURY = "POST_INJURY_RECOVERY"
FLOW_SUGGEST_AD_HOC = "SUGGEST_AD_HOC_SESSION"


# ─────────────────────────────────────────────────────────────────────────────
# Input / Output Schemas (Pydantic)
# ─────────────────────────────────────────────────────────────────────────────


class InjuryStatus(BaseModel):
    muscle_group: str
    severity: str  # "MILD" | "MODERATE" | "SEVERE"


class WorkoutSlot(BaseModel):
    day_of_week: int  # 0=Monday … 6=Sunday
    start_time: str   # "06:00"
    end_time: str     # "07:00"


class WorkoutSession(BaseModel):
    session_id: str
    completed_at: str       # ISO-8601
    muscle_groups: list[str]
    average_rpe: float
    average_form_score: float
    total_sets: int
    aborted: bool


class UserProfile(BaseModel):
    user_id: str
    weight_kg: float
    primary_goal: str
    available_equipment: list[str]
    preferred_muscle_groups: list[str]
    available_slots: list[WorkoutSlot]
    active_injuries: list[InjuryStatus]


class RoadmapSnapshot(BaseModel):
    roadmap_id: str
    current_week: int
    phase: str              # ACCUMULATION | OVERLOAD | PEAK | DELOAD
    pending_session_ids: list[str]


class CoachInput(BaseModel):
    """All mandatory context — assembled by @node fetch_user_context."""
    flow: str
    current_time: str       # ISO-8601 — for scheduling
    profile: UserProfile
    recent_sessions: list[WorkoutSession]
    current_roadmap: RoadmapSnapshot | None = None


# ── Generator output (text → parsed to this) ──────────────────────────────────

class PrescribedExercise(BaseModel):
    exercise_id: str
    exercise_name: str
    target_sets: int
    target_reps: int
    target_weight_kg: float
    target_rpe: float
    rest_set_sec: int


class WorkoutPrescription(BaseModel):
    warm_ups: list[PrescribedExercise]
    main_exercises: list[PrescribedExercise]
    cool_downs: list[PrescribedExercise]


class SessionPlan(BaseModel):
    session_plan_id: str
    scheduled_date: str     # YYYY-MM-DD
    target_muscle_groups: list[str]
    prescription: WorkoutPrescription
    reasoning: str


class WeekPlan(BaseModel):
    week_number: int        # 1–4
    phase: str              # ACCUMULATION | OVERLOAD | PEAK | DELOAD
    target_rpe_min: float
    target_rpe_max: float
    sessions: list[SessionPlan]


class GeneratedPlan(BaseModel):
    user_id: str
    weeks: list[WeekPlan] = Field(min_length=4, max_length=4)


# ── Evaluator output ──────────────────────────────────────────────────────────

class EvaluationResult(BaseModel):
    is_valid: bool
    issues: list[str]       # empty if valid
    plan: GeneratedPlan     # unchanged or corrected


# ─────────────────────────────────────────────────────────────────────────────
# Guardrail Callbacks (ADK v2)
# ─────────────────────────────────────────────────────────────────────────────

def validate_input_safety(ctx: Context, agent_input: CoachInput) -> dict | None:
    """Guardrail 1: before_model_callback (Input Validation).

    Chặn ngay từ đầu nếu tất cả các nhóm cơ mong muốn tập luyện của user
    đều đang bị chấn thương nặng (SEVERE).
    """
    profile = agent_input.profile
    severe_injuries = {
        inj.muscle_group.lower() for inj in profile.active_injuries
        if inj.severity == "SEVERE"
    }

    if severe_injuries and set(profile.preferred_muscle_groups).issubset(severe_injuries):
        return {
            "error": "Input Blocked: All preferred muscle groups have SEVERE injuries. Cannot generate program."
        }
    return None


def validate_tool_execution(ctx: Context, tool_name: str, args: dict) -> dict | None:
    """Guardrail 2: before_tool_callback (Tool Argument Validation).

    Chặn đứng nếu LLM cố tìm kiếm bài tập cho nhóm cơ đang chấn thương nặng.
    """
    coach_input = ctx.state.get("coach_input")
    if not coach_input:
        return None

    severe_injuries = {
        inj.muscle_group.lower() for inj in coach_input.profile.active_injuries
        if inj.severity == "SEVERE"
    }

    if tool_name == "search_exercises":
        target_muscle = args.get("muscle_group", "").lower()
        if target_muscle in severe_injuries:
            return {
                "error": f"Tool call blocked: muscle '{target_muscle}' has a SEVERE injury status."
            }
    return None


# ─────────────────────────────────────────────────────────────────────────────
# Dynamic Tools — LLM calls these during reasoning (not pre-fetched)
# ─────────────────────────────────────────────────────────────────────────────

def search_exercises(muscle_group: str, equipment: list[str], limit: int = 5) -> dict:
    """Search exercise catalog for a target muscle group and available equipment.

    LLM calls this when deciding which exercises to include in a session.

    Args:
        muscle_group: Target muscle — "chest"|"back"|"legs"|"shoulders"|"arms".
        equipment: User's available equipment list.
        limit: Max results to return.

    Returns:
        dict: {
            "exercises": [{"id", "name", "primary_muscle", "is_compound",
                           "equipment_required": list[str]}]
        }
    """
    _DB: dict[str, list[dict]] = {
        "chest": [
            {"id": "bench-press",  "name": "Bench Press",    "is_compound": True,  "equipment_required": ["barbell", "bench"]},
            {"id": "push-up",      "name": "Push Up",        "is_compound": True,  "equipment_required": []},
            {"id": "cable-fly",    "name": "Cable Fly",      "is_compound": False, "equipment_required": ["cable"]},
            {"id": "db-incline",   "name": "DB Incline",     "is_compound": True,  "equipment_required": ["dumbbell", "bench"]},
        ],
        "back": [
            {"id": "barbell-row",  "name": "Barbell Row",    "is_compound": True,  "equipment_required": ["barbell"]},
            {"id": "pull-up",      "name": "Pull Up",        "is_compound": True,  "equipment_required": ["pull-up-bar"]},
            {"id": "lat-pulldown", "name": "Lat Pulldown",   "is_compound": True,  "equipment_required": ["cable"]},
        ],
        "legs": [
            {"id": "squat",        "name": "Back Squat",     "is_compound": True,  "equipment_required": ["barbell", "rack"]},
            {"id": "rdl",          "name": "RDL",            "is_compound": True,  "equipment_required": ["barbell"]},
            {"id": "lunge",        "name": "Lunge",          "is_compound": True,  "equipment_required": []},
        ],
    }
    avail = set(e.lower() for e in equipment)
    candidates = [
        ex for ex in _DB.get(muscle_group.lower(), [])
        if not ex["equipment_required"] or set(ex["equipment_required"]).issubset(avail)
    ]
    return {
        "exercises": [
            {
                "id": ex["id"],
                "name": ex["name"],
                "primary_muscle": muscle_group,
                "is_compound": ex["is_compound"],
                "equipment_required": ex["equipment_required"],
            }
            for ex in candidates[:limit]
        ]
    }


def get_exercise_pr(exercise_id: str) -> dict:
    """Get user's personal record and recent weight history for weight calculation.

    LLM calls this to calculate safe target_weight_kg (must stay within ±30% of 1RM).

    Args:
        exercise_id: Exercise identifier.

    Returns:
        dict: {
            "exercise_id": str,
            "estimated_1rm_kg": float,
            "last_session_weight_kg": float,
            "trend": "improving"|"plateau"|"declining"
        }
    """
    _MOCK: dict[str, dict] = {
        "bench-press":  {"estimated_1rm_kg": 100.0, "last_session_weight_kg": 80.0,  "trend": "improving"},
        "squat":        {"estimated_1rm_kg": 140.0, "last_session_weight_kg": 110.0, "trend": "plateau"},
        "barbell-row":  {"estimated_1rm_kg": 90.0,  "last_session_weight_kg": 72.0,  "trend": "improving"},
        "rdl":          {"estimated_1rm_kg": 120.0, "last_session_weight_kg": 95.0,  "trend": "improving"},
    }
    pr = _MOCK.get(exercise_id, {"estimated_1rm_kg": 60.0, "last_session_weight_kg": 50.0, "trend": "plateau"})
    return {"exercise_id": exercise_id, **pr}


# ─────────────────────────────────────────────────────────────────────────────
# Static Skills (SkillToolset — Progressive Disclosure)
# ─────────────────────────────────────────────────────────────────────────────

coaching_rules_skill = Skill(
    frontmatter=Frontmatter(
        name="coaching-roadmap-rules",
        description="Detailed business rules for 4-week workout roadmap design (Accumulation, Overload, Peak, Deload) and output schemas.",
    ),
    instructions="""
    When designing a workout plan, follow these business rules strictly:
    
    1. Phase sequence and RPE targets:
       - Week 1: ACCUMULATION (Target RPE: 6.0 - 7.0)
       - Week 2: OVERLOAD     (Target RPE: 7.0 - 8.0)
       - Week 3: PEAK         (Target RPE: 8.0 - 9.0)
       - Week 4: DELOAD       (Target RPE: 5.0 - 6.0)
       
    2. Training Days constraints:
       - Do not prescribe more than 6 training days per week.
       - Rest days must be planned based on user's unavailable slots.
       
    3. Output validation guidelines:
       - Ensure all exercise IDs match search_exercises results.
       - Read references/schema-example.json for the correct JSON format.
    """,
    resources=Resources(
        references={
            "schema-example.json": """
            {
              "user_id": "user-123",
              "weeks": [
                {
                  "week_number": 1,
                  "phase": "ACCUMULATION",
                  "target_rpe_min": 6.0,
                  "target_rpe_max": 7.0,
                  "sessions": []
                }
              ]
            }
            """
        }
    )
)


# ─────────────────────────────────────────────────────────────────────────────
# @node — Mandatory Data Injection (deterministic, no LLM)
# ─────────────────────────────────────────────────────────────────────────────

@node
async def fetch_user_context(user_id: str, flow: str) -> CoachInput:
    """Fetch ALL mandatory context. Deterministic — pure data assembly."""
    now = datetime.now(timezone.utc)

    profile = UserProfile(
        user_id=user_id,
        weight_kg=75.0,
        primary_goal="hypertrophy",
        available_equipment=["barbell", "bench", "cable", "dumbbell", "pull-up-bar", "rack"],
        preferred_muscle_groups=["chest", "back", "legs", "shoulders"],
        available_slots=[
            WorkoutSlot(day_of_week=0, start_time="06:00", end_time="07:30"),
            WorkoutSlot(day_of_week=2, start_time="06:00", end_time="07:30"),
            WorkoutSlot(day_of_week=4, start_time="06:00", end_time="07:30"),
        ],
        active_injuries=[],
    )
    sessions: list[WorkoutSession] = []
    roadmap = None

    return CoachInput(
        flow=flow,
        current_time=now.isoformat(),
        profile=profile,
        recent_sessions=sessions,
        current_roadmap=roadmap,
    )


@node
def parse_to_schema(plan_text: str) -> GeneratedPlan:
    """Parse LLM text output → typed GeneratedPlan."""
    data = json.loads(plan_text)
    return GeneratedPlan(**data)


# ─────────────────────────────────────────────────────────────────────────────
# LLM Agents
# ─────────────────────────────────────────────────────────────────────────────

coaching_skill_toolset = SkillToolset(skills=[coaching_rules_skill])

_GENERATOR_INSTRUCTION = """\
You are CoachGeneratorAgent — expert AI fitness coach.

Your task is to generate or adapt a workout plan based on the input flow.
You must always load the 'coaching-roadmap-rules' skill to read the exact phase rules and schema structure before writing your output.

Flow tasks:
  INITIATE_4_WEEK    → generate 4 WeekPlans: ACCUMULATION→OVERLOAD→PEAK→DELOAD
  REGENERATE_PENDING → rewrite only PENDING sessions, keep COMPLETED unchanged
  ADAPTIVE_CYCLE     → adjust next week based on last week RPE/form metrics
  SIGNAL_HANDLER     → adjust sessions per behavioral signal (B1-B4)
  POST_INJURY        → cap weight at 50% 1RM for 3 sessions on recovered muscle
  SUGGEST_AD_HOC     → single session suggestion, read-only

Workflow:
  1. Call search_exercises(muscle_group, equipment) to find candidate exercises.
  2. Call get_exercise_pr(exercise_id) to set safe target_weight_kg.
  3. Load the coaching-roadmap-rules skill to verify phase target RPE and output schema.
  4. Write the final response in clean JSON matching the schema from the skill.

Output: valid JSON matching GeneratedPlan schema. No prose, no markdown code blocks.
"""

_EVALUATOR_INSTRUCTION = """\
You are CoachEvaluatorAgent — final quality reviewer for coaching plans.

Review the GeneratedPlan provided as input:
  1. Exactly 4 weeks in order: ACCUMULATION → OVERLOAD → PEAK → DELOAD
  2. RPE per phase within bounds: 6-7 / 7-8 / 8-9 / 5-6
  3. Max 6 sessions per week in any week
  4. DELOAD total sets ≤ 70% of PEAK week total sets
  5. No session targets muscle groups in active_injuries
  6. Weights within ±30% of estimated 1RM

Decision:
  - ALL checks pass → is_valid=True, issues=[], return plan unchanged
  - ANY check fails → is_valid=False, fix violations, return corrected plan + list issues
"""

generator_agent = Agent(
    name="CoachGeneratorAgent",
    model=_MODEL,
    instruction=_GENERATOR_INSTRUCTION,
    tools=[search_exercises, get_exercise_pr, coaching_skill_toolset],
    output_key="generated_plan_text",
    before_model_callback=validate_input_safety,  # Guardrail 1
    before_tool_callback=validate_tool_execution,   # Guardrail 2
)

evaluator_agent = Agent(
    name="CoachEvaluatorAgent",
    model=_MODEL,
    instruction=_EVALUATOR_INSTRUCTION,
    input_schema=GeneratedPlan,
    output_schema=EvaluationResult,
)


# ─────────────────────────────────────────────────────────────────────────────
# @node Workflows
# ─────────────────────────────────────────────────────────────────────────────

@node
async def init_roadmap_workflow(ctx: Context, user_id: str) -> EvaluationResult:
    """FlowInitiate4Week: inject → generate (with tools/skills/callbacks) → parse → evaluate."""
    coach_input = await ctx.run_node(fetch_user_context, user_id, FLOW_INITIATE_4_WEEK)
    # Lưu coach_input vào context state để before_tool_callback có thể truy cập
    ctx.state["coach_input"] = coach_input

    plan_text: str = await ctx.run_node(generator_agent, coach_input)
    generated: GeneratedPlan = await ctx.run_node(parse_to_schema, plan_text)
    return await ctx.run_node(evaluator_agent, generated)


@node
async def default_workflow(ctx: Context, user_id: str, flow: str) -> GeneratedPlan:
    """All other flows: inject → generate → parse."""
    coach_input = await ctx.run_node(fetch_user_context, user_id, flow)
    ctx.state["coach_input"] = coach_input

    plan_text: str = await ctx.run_node(generator_agent, coach_input)
    return await ctx.run_node(parse_to_schema, plan_text)


# ─────────────────────────────────────────────────────────────────────────────
# Entry Point
# ─────────────────────────────────────────────────────────────────────────────

class CoachingContextAgent:
    async def run(self, user_id: str, flow: str) -> EvaluationResult | GeneratedPlan:
        ctx = Context()
        if flow == FLOW_INITIATE_4_WEEK:
            return await ctx.run_node(init_roadmap_workflow, user_id)
        return await ctx.run_node(default_workflow, user_id, flow)
