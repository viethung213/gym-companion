package taxonomy

import (
	"errors"
	"testing"
)

func TestParse(t *testing.T) {
	if got, err := ParseJointRegion("ANKLE_FOOT"); err != nil || got != JointRegionAnkleFoot {
		t.Errorf("ParseJointRegion(ANKLE_FOOT) = %q, %v", got, err)
	}
	if got, err := ParseInjurySeverity("SEVERE"); err != nil || got != InjurySeveritySevere {
		t.Errorf("ParseInjurySeverity(SEVERE) = %q, %v", got, err)
	}

	invalid := []func() error{
		func() error { _, err := ParseMuscleGroup("Pectorals"); return err },
		func() error { _, err := ParseEquipment("body only"); return err },
		func() error { _, err := ParseCategory(""); return err },
		func() error { _, err := ParseLevel("ADVANCED"); return err },
		func() error { _, err := ParseForce("TWIST"); return err },
		func() error { _, err := ParseMechanic("BOTH"); return err },
		func() error { _, err := ParseJointRegion("Knee"); return err },
		func() error { _, err := ParseInjurySeverity("CRITICAL"); return err },
	}
	for i, parse := range invalid {
		if err := parse(); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("case %d: got %v, want ErrUnknownValue", i, err)
		}
	}
}
