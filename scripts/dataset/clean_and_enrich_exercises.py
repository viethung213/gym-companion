import json
import os
import re
import uuid
from difflib import SequenceMatcher

def slugify(text):
    text = text.lower().strip()
    text = re.sub(r'[\/\s\-_]+', '_', text)
    text = re.sub(r'[^\w_]', '', text)
    return text.strip('_')

def title_case(name):
    words = name.strip().split()
    minor_words = {'a', 'an', 'and', 'as', 'at', 'but', 'by', 'for', 'in', 'nor', 'of', 'on', 'or', 'so', 'the', 'to', 'up', 'yet', 'with', 'on'}
    result = []
    for i, w in enumerate(words):
        lw = w.lower()
        if i == 0 or lw not in minor_words or w.startswith('('):
            result.append(w.capitalize())
        else:
            result.append(lw)
    return ' '.join(result)

def main():
    dataset_path = r'E:\LEARN\a\exercises-dataset\data\exercises.json'
    scratch_yt_path = r'scratch_channel_videos.json'
    
    with open(dataset_path, 'r', encoding='utf-8') as f:
        raw_exercises = json.load(f)
        
    yt_videos = []
    if os.path.exists(scratch_yt_path):
        with open(scratch_yt_path, 'r', encoding='utf-8') as f:
            yt_videos = json.load(f)

    # 1. Prepare YouTube video match dictionary
    def clean_yt_title(t):
        t = re.sub(r'\|.*', '', t)
        t = re.sub(r'\(.*?\)', '', t)
        t = re.sub(r'[^\w\s-]', '', t)
        return t.strip().lower()

    default_fallback_video = 'https://www.youtube.com/watch?v=1nI2aCxjVKs'
    category_fallbacks = {
        'waist': 'https://www.youtube.com/watch?v=yLZ4FoGeqJE',
        'upper legs': 'https://www.youtube.com/watch?v=wFOgKtxNCiI',
        'lower legs': 'https://www.youtube.com/watch?v=ZwLo_6yH8Wo',
        'back': 'https://www.youtube.com/watch?v=m7mwLAyR3k4',
        'chest': 'https://www.youtube.com/watch?v=bSkNEdRaJlg',
        'shoulders': 'https://www.youtube.com/watch?v=AJZRMJ7qlbo',
        'upper arms': 'https://www.youtube.com/watch?v=pjo7EMsSWwc',
        'lower arms': 'https://www.youtube.com/watch?v=AzpNgSRmM0M',
        'cardio': 'https://www.youtube.com/watch?v=XeUpLdzdbAE',
        'neck': 'https://www.youtube.com/watch?v=JZAWzuYdH6Y',
    }

    matched_yt = {}
    for vid in yt_videos:
        v_title = vid.get('title', '')
        v_id = vid.get('id', '')
        c_title = clean_yt_title(v_title)
        if not c_title or not v_id:
            continue
        v_url = f'https://www.youtube.com/watch?v={v_id}'
        
        for ex in raw_exercises:
            ex_name = ex['name'].lower()
            ratio = SequenceMatcher(None, c_title, ex_name).ratio()
            t_words = set(c_title.split())
            e_words = set(ex_name.split())
            if t_words and e_words and (t_words == e_words or (t_words.issubset(e_words) and len(t_words) >= 2)):
                ratio = max(ratio, 0.90)
            if ratio >= 0.70:
                if ex['id'] not in matched_yt or ratio > matched_yt[ex['id']][1]:
                    matched_yt[ex['id']] = (v_url, ratio, v_title)

    # 2. Metadata Catalogs
    body_parts_map = {
        'waist': 'Waist',
        'upper legs': 'Upper Legs',
        'back': 'Back',
        'lower legs': 'Lower Legs',
        'chest': 'Chest',
        'upper arms': 'Upper Arms',
        'cardio': 'Cardio',
        'shoulders': 'Shoulders',
        'lower arms': 'Lower Arms',
        'neck': 'Neck'
    }

    body_parts_list = [
        {'id': f'bp_{slugify(bp)}', 'name': name, 'slug': slugify(bp)}
        for bp, name in body_parts_map.items()
    ]

    equipments_set = set()
    for ex in raw_exercises:
        equipments_set.add(ex['equipment'])

    equipment_names = {
        'body weight': 'Body Weight',
        'barbell': 'Barbell',
        'dumbbell': 'Dumbbell',
        'cable': 'Cable',
        'leverage machine': 'Leverage Machine',
        'band': 'Band',
        'smith machine': 'Smith Machine',
        'kettlebell': 'Kettlebell',
        'weighted': 'Weighted',
        'stability ball': 'Stability Ball',
        'ez barbell': 'EZ Barbell',
        'assisted': 'Assisted',
        'sled machine': 'Sled Machine',
        'medicine ball': 'Medicine Ball',
        'rope': 'Rope',
        'roller': 'Roller',
        'resistance band': 'Resistance Band',
        'bosu ball': 'Bosu Ball',
        'olympic barbell': 'Olympic Barbell',
        'wheel roller': 'Wheel Roller',
        'upper body ergometer': 'Upper Body Ergometer',
        'skierg machine': 'SkiErg Machine',
        'hammer': 'Hammer',
        'stationary bike': 'Stationary Bike',
        'tire': 'Tire',
        'trap bar': 'Trap Bar',
        'elliptical machine': 'Elliptical Machine',
        'stepmill machine': 'Stepmill Machine'
    }

    equipments_list = []
    for eq in sorted(list(equipments_set)):
        eq_slug = slugify(eq)
        name = equipment_names.get(eq, title_case(eq))
        equipments_list.append({'id': f'eq_{eq_slug}', 'name': name, 'slug': eq_slug})

    canonical_muscle_bp = {
        'abs': 'bp_waist',
        'hip flexors': 'bp_waist',
        'lower back': 'bp_waist',
        'obliques': 'bp_waist',
        'back': 'bp_waist',
        'core': 'bp_waist',
        'quads': 'bp_upper_legs',
        'quadriceps': 'bp_upper_legs',
        'hamstrings': 'bp_upper_legs',
        'glutes': 'bp_upper_legs',
        'adductors': 'bp_upper_legs',
        'abductors': 'bp_upper_legs',
        'lats': 'bp_back',
        'rhomboids': 'bp_back',
        'spine': 'bp_back',
        'upper back': 'bp_back',
        'rear deltoids': 'bp_back',
        'traps': 'bp_back',
        'calves': 'bp_lower_legs',
        'ankle stabilizers': 'bp_lower_legs',
        'ankles': 'bp_lower_legs',
        'feet': 'bp_lower_legs',
        'pectorals': 'bp_chest',
        'chest': 'bp_chest',
        'serratus anterior': 'bp_chest',
        'deltoids': 'bp_chest',
        'biceps': 'bp_upper_arms',
        'triceps': 'bp_upper_arms',
        'delts': 'bp_shoulders',
        'shoulders': 'bp_shoulders',
        'trapezius': 'bp_shoulders',
        'forearms': 'bp_lower_arms',
        'cardiovascular system': 'bp_cardio',
        'levator scapulae': 'bp_neck'
    }

    all_muscles_set = set()
    for ex in raw_exercises:
        if ex.get('target'):
            all_muscles_set.add(ex['target'])
        if ex.get('muscle_group'):
            all_muscles_set.add(ex['muscle_group'])
        for sm in ex.get('secondary_muscles', []):
            all_muscles_set.add(sm)

    muscles_list = []
    for m in sorted(list(all_muscles_set)):
        m_slug = slugify(m)
        bp_id = canonical_muscle_bp.get(m, f"bp_{slugify(raw_exercises[0]['body_part'])}")
        muscles_list.append({
            'id': f'muscle_{m_slug}',
            'name': title_case(m),
            'body_part_id': bp_id
        })

    tags_list = [
        {'id': 'tag_compound', 'name': 'Compound Movement'},
        {'id': 'tag_isolation', 'name': 'Isolation Movement'},
        {'id': 'tag_strength', 'name': 'Strength & Hypertrophy'},
        {'id': 'tag_bodyweight', 'name': 'Bodyweight & Calisthenics'},
        {'id': 'tag_home_workout', 'name': 'Home Workout Friendly'},
        {'id': 'tag_machine', 'name': 'Machine Exercise'},
        {'id': 'tag_free_weights', 'name': 'Free Weights'},
        {'id': 'tag_cardio', 'name': 'Cardio & Stamina'},
        {'id': 'tag_mobility', 'name': 'Mobility & Warm-up'},
        {'id': 'tag_core_stability', 'name': 'Core & Stability'}
    ]
    for bp in body_parts_list:
        tags_list.append({
            'id': f"tag_category_{bp['slug']}",
            'name': f"Category: {bp['name']}"
        })

    name_counter = {}
    for ex in raw_exercises:
        n = ex['name'].strip().lower()
        name_counter[n] = name_counter.get(n, 0) + 1

    name_seen = {}
    clean_exercises = []

    ai_supported_patterns = [
        r'\bsquat\b', r'\bpush-up\b', r'\bpushup\b', r'\bcurl\b', r'\blunge\b',
        r'\blateral raise\b', r'\bshoulder press\b', r'\bplank\b', r'\bsit-up\b',
        r'\bcrunch\b', r'\bmountain climber\b', r'\bglute bridge\b', r'\bdip\b',
        r'\bdeadlift\b', r'\bbench press\b', r'\bpull-up\b', r'\bchin-up\b'
    ]

    for ex in raw_exercises:
        raw_id = ex['id']
        raw_name = ex['name'].strip().lower()
        
        # Random UUID v4 for exercise id
        exercise_uuid = str(uuid.uuid4())

        if name_counter[raw_name] > 1:
            idx = name_seen.get(raw_name, 0) + 1
            name_seen[raw_name] = idx
            formatted_name = f"{title_case(raw_name)} (Variation {idx})"
        else:
            formatted_name = title_case(raw_name)

        bp_slug = slugify(ex['body_part'])
        eq_slug = slugify(ex['equipment'])
        target_slug = slugify(ex['target'])
        
        sec_muscles = [f"muscle_{slugify(sm)}" for sm in ex.get('secondary_muscles', [])]
        
        ex_tags = [f"tag_category_{bp_slug}"]
        if ex['equipment'] == 'body weight':
            ex_tags.extend(['tag_bodyweight', 'tag_home_workout'])
        elif 'machine' in ex['equipment'] or ex['equipment'] in ['cable', 'leverage machine', 'smith machine']:
            ex_tags.append('tag_machine')
        elif ex['equipment'] in ['barbell', 'dumbbell', 'kettlebell', 'ez barbell', 'olympic barbell']:
            ex_tags.append('tag_free_weights')

        if len(sec_muscles) >= 2 or ex['body_part'] in ['upper legs', 'back', 'chest']:
            ex_tags.append('tag_compound')
        else:
            ex_tags.append('tag_isolation')

        if ex['body_part'] == 'waist':
            ex_tags.append('tag_core_stability')
        elif ex['body_part'] == 'cardio':
            ex_tags.append('tag_cardio')

        eq = ex['equipment']
        if eq in ['olympic barbell', 'trap bar'] or any(k in raw_name for k in ['snatch', 'clean', 'muscle-up', 'one-arm', 'weighted pull-up', 'dragon flag']):
            difficulty = 'Advanced'
        elif eq in ['barbell', 'kettlebell'] or any(k in raw_name for k in ['deadlift', 'squat', 'bench press', 'overhead press', 'pull-up', 'dip']):
            difficulty = 'Intermediate'
        else:
            difficulty = 'Beginner'

        if difficulty == 'Advanced' or any(k in raw_name for k in ['deadlift', 'squat', 'bench press']):
            rest_sec = 120
        elif 'compound' in ex_tags:
            rest_sec = 90
        elif ex['body_part'] in ['waist', 'cardio', 'lower legs', 'lower arms']:
            rest_sec = 45
        else:
            rest_sec = 60

        has_ai = any(re.search(pat, raw_name) for pat in ai_supported_patterns)

        steps = ex.get('instruction_steps', {}).get('en', [])
        if not steps and isinstance(ex.get('instructions'), dict):
            en_inst = ex['instructions'].get('en', '')
            steps = [s.strip() for s in en_inst.split('.') if s.strip()]

        numbered_instructions = '\n'.join([f"{i+1}. {st.strip()}" for i, st in enumerate(steps)])

        media_id = ex.get('media_id', '')
        media_url = f"https://static.exercisedb.dev/media/{media_id}.gif" if media_id else None
        thumbnail_url = media_url

        if raw_id in matched_yt:
            video_url = matched_yt[raw_id][0]
        else:
            video_url = category_fallbacks.get(ex['body_part'], default_fallback_video)

        clean_item = {
            'id': exercise_uuid,
            'original_id': raw_id,
            'name': formatted_name,
            'body_part_id': f"bp_{bp_slug}",
            'equipment_id': f"eq_{eq_slug}",
            'target_muscle_id': f"muscle_{target_slug}",
            'secondary_muscle_ids': sec_muscles,
            'tag_ids': ex_tags,
            'instructions': numbered_instructions,
            'instruction_steps': ex.get('instruction_steps', {}),
            'media_id': media_id,
            'thumbnail_url': thumbnail_url,
            'media_url': media_url,
            'video_url': video_url,
            'difficulty': difficulty,
            'default_rest_seconds': rest_sec,
            'status': 'ACTIVE',
            'has_ai_supported': has_ai
        }
        clean_exercises.append(clean_item)

    final_dataset = {
        'metadata': {
            'total_exercises': len(clean_exercises),
            'body_parts_count': len(body_parts_list),
            'equipments_count': len(equipments_list),
            'muscles_count': len(muscles_list),
            'tags_count': len(tags_list),
            'body_parts': body_parts_list,
            'equipments': equipments_list,
            'muscles': muscles_list,
            'tags': tags_list
        },
        'exercises': clean_exercises
    }

    os.makedirs(r'E:\LEARN\a\gym-companion\internal\shared\database\seeds\data', exist_ok=True)
    out_file1 = r'E:\LEARN\a\gym-companion\internal\shared\database\seeds\data\clean_exercises.json'
    out_file2 = r'E:\LEARN\a\exercises-dataset\data\clean_exercises.json'

    with open(out_file1, 'w', encoding='utf-8') as f:
        json.dump(final_dataset, f, ensure_ascii=False, indent=2)
    print(f'Successfully wrote clean dataset with Random UUIDs to: {out_file1}')

    with open(out_file2, 'w', encoding='utf-8') as f:
        json.dump(final_dataset, f, ensure_ascii=False, indent=2)
    print(f'Successfully wrote clean dataset with Random UUIDs to: {out_file2}')

    # Also update jsonl files
    out_jsonl_1 = r'E:\LEARN\a\gym-companion\internal\shared\database\seeds\data\clean_exercises.jsonl'
    out_jsonl_2 = r'E:\LEARN\a\exercises-dataset\data\clean_exercises.jsonl'
    with open(out_jsonl_1, 'w', encoding='utf-8') as f1, open(out_jsonl_2, 'w', encoding='utf-8') as f2:
        for item in clean_exercises:
            l = json.dumps(item, ensure_ascii=False) + '\n'
            f1.write(l)
            f2.write(l)
    print('Successfully updated clean_exercises.jsonl files!')

if __name__ == '__main__':
    main()
