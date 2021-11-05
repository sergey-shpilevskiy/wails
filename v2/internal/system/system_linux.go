//go:build linux
// +build linux

package system

import (
	"github.com/sergey-shpilevskiy/wails/v2/internal/system/operatingsystem"
	"github.com/sergey-shpilevskiy/wails/v2/internal/system/packagemanager"
)

func (i *Info) discover() error {

	var err error
	osinfo, err := operatingsystem.Info()
	if err != nil {
		return err
	}
	i.OS = osinfo

	i.PM = packagemanager.Find(osinfo.ID)
	if i.PM != nil {
		dependencies, err := packagemanager.Dependancies(i.PM)
		if err != nil {
			return err
		}
		i.Dependencies = dependencies
	}

	return nil
}
