package dotnetframework

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/cloudfoundry/libbuildpack"
)

type Installer interface {
	InstallDependency(libbuildpack.Dependency, string) error
}

type DotnetFramework struct {
	depDir    string
	installer Installer
	manifest  *libbuildpack.Manifest
	logger    *libbuildpack.Logger
	buildDir  string
}

func New(depDir string, buildDir string, installer Installer, manifest *libbuildpack.Manifest, logger *libbuildpack.Logger) *DotnetFramework {
	return &DotnetFramework{
		depDir:    depDir,
		installer: installer,
		manifest:  manifest,
		logger:    logger,
		buildDir:  buildDir,
	}
}

func (d *DotnetFramework) Install() error {
	versions, err := d.requiredVersions()
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return nil
	}
	d.logger.Info("Required dotnetframework versions: %v", versions)

	for _, v := range versions {
		if found, err := d.isInstalled(v); err != nil {
			return err
		} else if !found {
			if err := d.installFramework(v); err != nil {
				return err
			}
		}
	}
	return nil
}

func (d *DotnetFramework) requiredVersions() ([]string, error) {
	runtimeFile, err := d.runtimeConfigFile()
	if err != nil {
		return []string{}, err
	}
	if runtimeFile != "" {
		obj := struct {
			RuntimeOptions struct {
				Framework struct {
					Name    string `json:"name"`
					Version string `json:"version"`
				} `json:"framework"`
				ApplyPatches *bool `json:"applyPatches"`
			} `json:"runtimeOptions"`
		}{}

		if err := libbuildpack.NewJSON().Load(runtimeFile, &obj); err != nil {
			return []string{}, err
		}
		version := obj.RuntimeOptions.Framework.Version
		if version != "" {
			if obj.RuntimeOptions.ApplyPatches == nil || *obj.RuntimeOptions.ApplyPatches {
				v := strings.Split(version, ".")
				v[2] = "x"
				versions := d.manifest.AllDependencyVersions("dotnet-framework")
				version, err = libbuildpack.FindMatchingVersion(strings.Join(v, "."), versions)
				if err != nil {
					return []string{}, err
				}
			}
			return []string{version}, nil
		}
		return []string{}, nil
	}
	restoredVersionsDir := filepath.Join(d.depDir, ".nuget", "packages", "microsoft.netcore.app")
	if exists, err := libbuildpack.FileExists(restoredVersionsDir); err != nil {
		return []string{}, err
	} else if !exists {
		return []string{}, nil
	}
	files, err := ioutil.ReadDir(restoredVersionsDir)
	if err != nil {
		return []string{}, err
	}
	var versions []string
	for _, f := range files {
		versions = append(versions, f.Name())
	}
	return versions, nil
}

func (d *DotnetFramework) getFrameworkDir() string {
	return filepath.Join(d.depDir, "dotnet", "shared", "Microsoft.NETCore.App")
}

func (d *DotnetFramework) isInstalled(version string) (bool, error) {
	frameworkPath := filepath.Join(d.getFrameworkDir(), version)
	if exists, err := libbuildpack.FileExists(frameworkPath); err != nil {
		return false, err
	} else if exists {
		d.logger.Info("Using dotnet framework installed in %s", frameworkPath)
		return true, nil
	}
	return false, nil
}

func (d *DotnetFramework) installFramework(version string) error {
	if err := d.installer.InstallDependency(libbuildpack.Dependency{Name: "dotnet-framework", Version: version}, filepath.Join(d.depDir, "dotnet")); err != nil {
		return err
	}
	return nil
}

func (d *DotnetFramework) runtimeConfigFile() (string, error) {
	if configFiles, err := filepath.Glob(filepath.Join(d.buildDir, "*.runtimeconfig.json")); err != nil {
		return "", err
	} else if len(configFiles) == 1 {
		return configFiles[0], nil
	} else if len(configFiles) > 1 {
		return "", fmt.Errorf("Multiple .runtimeconfig.json files present")
	}
	return "", nil
}
