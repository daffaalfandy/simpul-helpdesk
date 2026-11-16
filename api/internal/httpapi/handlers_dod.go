package httpapi

import (
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/dod"
	"github.com/daffaalfandy/simpul-helpdesk/api/internal/httpapi/gen"
)

func apiDodConfig(c dod.Config) gen.DodConfig {
	return gen.DodConfig{
		Merged:             gen.DodMerged{Enabled: c.Merged.Enabled, Branch: gen.DodBranch(c.Merged.Branch)},
		CiPassed:           c.CIPassed,
		DeployedProduction: c.DeployedProduction,
		Uat:                gen.DodUat{Enabled: c.UAT.Enabled, AutoApproveWorkingDays: gen.AutoApproveDays(c.UAT.AutoApproveWorkingDays)},
		RequesterConfirms:  c.RequesterConfirms,
	}
}

func domainDodConfig(c gen.DodConfig) dod.Config {
	return dod.Config{
		Merged:             dod.Merged{Enabled: c.Merged.Enabled, Branch: dod.Branch(c.Merged.Branch)},
		CIPassed:           c.CiPassed,
		DeployedProduction: c.DeployedProduction,
		UAT:                dod.UAT{Enabled: c.Uat.Enabled, AutoApproveWorkingDays: int(c.Uat.AutoApproveWorkingDays)},
		RequesterConfirms:  c.RequesterConfirms,
	}
}
