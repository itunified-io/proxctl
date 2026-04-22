package license

import "testing"

func TestLicense_TierConstants(t *testing.T) {
	if TierCommunity.String() != "community" {
		t.Errorf("TierCommunity.String() = %q, want community", TierCommunity.String())
	}
	if TierBusiness.String() != "business" {
		t.Errorf("TierBusiness.String() = %q, want business", TierBusiness.String())
	}
	if TierEnterprise.String() != "enterprise" {
		t.Errorf("TierEnterprise.String() = %q, want enterprise", TierEnterprise.String())
	}
}

func TestLicense_CatalogHasCoreTools(t *testing.T) {
	core := []string{"vm.create", "vm.delete", "workflow.up", "kickstart.generate", "license.status"}
	for _, tool := range core {
		if _, ok := ToolCatalog[tool]; !ok {
			t.Errorf("ToolCatalog missing core tool %q", tool)
		}
	}
}

func TestLicense_CheckCommunityAlwaysAllowed(t *testing.T) {
	if err := Check("vm.create"); err != nil {
		t.Errorf("Check(vm.create) should pass at Community, got %v", err)
	}
}
