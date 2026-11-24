package protomap

import (
	"errors"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	exercisemsg "github.com/viethung213/gym-companion/internal/gen/go/contracts/supporting/exercise/v1/message"
	"github.com/viethung213/gym-companion/internal/profile/domain/derror"
	"github.com/viethung213/gym-companion/internal/profile/domain/vo"
)

const profileMigrationPath = "../../../shared/database/migrations/core/04-profile.sql"

// TestTaxonomy_NoDrift enforces docs/05 §7: domain VO set == proto enum set
// (prefix and UNSPECIFIED stripped) == migration CHECK set. Equipment and
// preferred muscle groups live in JSONB, so they are validated by the domain only.
func TestTaxonomy_NoDrift(t *testing.T) {
	migration, err := os.ReadFile(profileMigrationPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}

	tests := []struct {
		name       string
		domain     []string
		proto      map[string]int32
		prefix     string
		constraint string
	}{
		{"joint region", codes(vo.AllJointRegions()), exercisemsg.JointRegion_value, prefixJointRegion, "chk_injuries_region"},
		{"muscle group", codes(vo.AllMuscleGroups()), exercisemsg.MuscleGroup_value, prefixMuscleGroup, "chk_injuries_muscle"},
		{"body side", codes(vo.AllBodySides()), exercisemsg.BodySide_value, prefixBodySide, "chk_injuries_side"},
		{"injury severity", codes(vo.AllInjurySeverities()), exercisemsg.InjurySeverity_value, prefixInjurySeverity, "chk_injuries_severity"},
		{"equipment", codes(vo.AllEquipment()), exercisemsg.Equipment_value, prefixEquipment, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := sorted(tt.domain)
			if got := protoCodes(tt.proto, tt.prefix); !equal(got, want) {
				t.Errorf("proto codes %v, want domain codes %v", got, want)
			}
			if tt.constraint == "" {
				return
			}
			if got := checkCodes(t, string(migration), tt.constraint); !equal(got, want) {
				t.Errorf("migration %s codes %v, want domain codes %v", tt.constraint, got, want)
			}
		})
	}
}

func TestFromProto(t *testing.T) {
	tests := []struct {
		name    string
		give    func() (string, error)
		want    string
		wantErr bool
	}{
		{"region", func() (string, error) {
			v, err := JointRegionFromProto(exercisemsg.JointRegion_JOINT_REGION_KNEE)
			return string(v), err
		}, "KNEE", false},
		{"region unspecified", func() (string, error) {
			v, err := JointRegionFromProto(exercisemsg.JointRegion_JOINT_REGION_UNSPECIFIED)
			return string(v), err
		}, "", true},
		{"region unknown number", func() (string, error) {
			v, err := JointRegionFromProto(exercisemsg.JointRegion(77))
			return string(v), err
		}, "", true},
		{"optional muscle unspecified", func() (string, error) {
			v, err := OptionalMuscleGroupFromProto(exercisemsg.MuscleGroup_MUSCLE_GROUP_UNSPECIFIED)
			return string(v), err
		}, "", false},
		{"side defaults to both", func() (string, error) {
			v, err := BodySideFromProto(exercisemsg.BodySide_BODY_SIDE_UNSPECIFIED)
			return string(v), err
		}, "BOTH", false},
		{"severity unspecified", func() (string, error) {
			v, err := InjurySeverityFromProto(exercisemsg.InjurySeverity_INJURY_SEVERITY_UNSPECIFIED)
			return string(v), err
		}, "", true},
		{"equipment list with unknown", func() (string, error) {
			_, err := EquipmentListFromProto([]exercisemsg.Equipment{exercisemsg.Equipment_EQUIPMENT_CABLE, 40})
			return "", err
		}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.give()
			if tt.wantErr {
				if !errors.Is(err, derror.ErrInvalidTaxonomy) {
					t.Errorf("got error %v, want ErrInvalidTaxonomy", err)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func codes[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

func protoCodes(values map[string]int32, prefix string) []string {
	var out []string
	for name, number := range values {
		if number != 0 {
			out = append(out, strings.TrimPrefix(name, prefix))
		}
	}
	return sorted(out)
}

var quotedCode = regexp.MustCompile(`'([^']+)'`)

// checkCodes returns the quoted literals inside the named CHECK constraint.
func checkCodes(t *testing.T, sql, constraint string) []string {
	t.Helper()

	marker := "CONSTRAINT " + constraint + " CHECK "
	start := strings.Index(sql, marker)
	if start < 0 {
		t.Fatalf("constraint %s not found", constraint)
	}
	body := sql[start+len(marker):]
	depth, end := 0, -1
	for i, r := range body {
		if r == '(' {
			depth++
		}
		if r == ')' {
			if depth--; depth == 0 {
				end = i
				break
			}
		}
	}
	if end < 0 {
		t.Fatalf("constraint %s is not closed", constraint)
	}

	matches := quotedCode.FindAllStringSubmatch(body[:end], -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		out = append(out, m[1])
	}
	return sorted(out)
}

func sorted(vs []string) []string {
	out := append([]string(nil), vs...)
	sort.Strings(out)
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
