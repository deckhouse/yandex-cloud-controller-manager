package yandex

import "testing"

func TestParseProviderID(t *testing.T) {
	const (
		deprecatedProviderID = "yandex://folder/zone/testid"
		providerID           = "yandex://testid"
		invalidProviderID    = "mail://test"
	)

	name, instanceNameIsId, err := ParseProviderID(deprecatedProviderID)
	if err != nil {
		t.Error(err)
	}
	if instanceNameIsId {
		t.Error("deprecatedProviderID should not return positive instanceNameIsId")
	}
	if len(name) == 0 {
		t.Error("name field is empty")
	}

	id, instanceNameIsId, err := ParseProviderID(providerID)
	if err != nil {
		t.Error(err)
	}
	if !instanceNameIsId {
		t.Error("deprecatedProviderID should return positive instanceNameIsId")
	}
	if len(id) == 0 {
		t.Error("id field is empty")
	}

	_, _, err = ParseProviderID(invalidProviderID)
	if err == nil {
		t.Error("should return non-nil err on invalid ProviderID")
	}
}
