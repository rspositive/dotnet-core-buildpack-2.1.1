package brats_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/bratshelper"
	"github.com/cloudfoundry/libbuildpack/cutlass"

	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func init() {
	flag.StringVar(&cutlass.DefaultMemory, "memory", "256M", "default memory for pushed apps")
	flag.StringVar(&cutlass.DefaultDisk, "disk", "512M", "default disk for pushed apps")
	flag.Parse()
}

var _ = SynchronizedBeforeSuite(func() []byte {
	// Run once
	return bratshelper.InitBpData(os.Getenv("CF_STACK")).Marshal()
}, func(data []byte) {
	// Run on all nodes
	bratshelper.Data.Unmarshal(data)
	Expect(cutlass.CopyCfHome()).To(Succeed())
	cutlass.SeedRandom()
	cutlass.DefaultStdoutStderr = GinkgoWriter
})

var _ = SynchronizedAfterSuite(func() {
	// Run on all nodes
}, func() {
	// Run once
	Expect(cutlass.DeleteOrphanedRoutes()).To(Succeed())
	Expect(cutlass.DeleteBuildpack(strings.Replace(bratshelper.Data.Cached, "_buildpack", "", 1))).To(Succeed())
	Expect(cutlass.DeleteBuildpack(strings.Replace(bratshelper.Data.Uncached, "_buildpack", "", 1))).To(Succeed())
	Expect(os.Remove(bratshelper.Data.CachedFile)).To(Succeed())
	Expect(os.Remove(bratshelper.Data.UncachedFile)).To(Succeed())
})

func TestBrats(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Brats Suite")
}

func FirstOfVersionLine(dependency, line string) string {
	bpDir, err := cutlass.FindRoot()
	if err != nil {
		panic(err)
	}
	manifest, err := libbuildpack.NewManifest(bpDir, nil, time.Now())
	if err != nil {
		panic(err)
	}
	deps := manifest.AllDependencyVersions(dependency)
	versions, err := libbuildpack.FindMatchingVersions(line, deps)
	if err != nil {
		panic(err)
	}
	return versions[0]
}

func CopyBratsWithFramework(sdkVersion, frameworkVersion string) *cutlass.App {
	manifest, err := libbuildpack.NewManifest(bratshelper.Data.BpDir, nil, time.Now())
	Expect(err).ToNot(HaveOccurred())

	if sdkVersion == "" {
		sdkVersion = "x"
	}
	if strings.Contains(sdkVersion, "x") {
		deps := manifest.AllDependencyVersions("dotnet")
		sdkVersion, err = libbuildpack.FindMatchingVersion(sdkVersion, deps)
		Expect(err).ToNot(HaveOccurred())
	}

	if frameworkVersion == "" {
		frameworkVersion = "x"
	}
	if strings.Contains(frameworkVersion, "x") {
		deps := manifest.AllDependencyVersions("dotnet-framework")
		frameworkVersion, err = libbuildpack.FindMatchingVersion(frameworkVersion, deps)
		Expect(err).ToNot(HaveOccurred())
	}

	versionParts := strings.Split(frameworkVersion, ".")
	netCoreApp := fmt.Sprintf("netcoreapp%s.%s", versionParts[0], versionParts[1])

	dir, err := cutlass.CopyFixture(filepath.Join(bratshelper.Data.BpDir, "fixtures", "simple_brats"))
	Expect(err).ToNot(HaveOccurred())

	for _, file := range []string{"simple_brats.csproj", "global.json"} {
		data, err := ioutil.ReadFile(filepath.Join(dir, file))
		Expect(err).ToNot(HaveOccurred())
		data = bytes.Replace(data, []byte("<%= net_core_app %>"), []byte(netCoreApp), -1)
		data = bytes.Replace(data, []byte("<%= framework_version %>"), []byte(frameworkVersion), -1)
		data = bytes.Replace(data, []byte("<%= sdk_version %>"), []byte(sdkVersion), -1)
		Expect(ioutil.WriteFile(filepath.Join(dir, file), data, 0644)).To(Succeed())
	}

	return cutlass.New(dir)
}

func CopyBrats(sdkVersion string) *cutlass.App {
	return CopyBratsWithFramework(sdkVersion, "")
}

func PushApp(app *cutlass.App) {
	Expect(app.Push()).To(Succeed())
	Eventually(app.InstanceStates, 20*time.Second).Should(Equal([]string{"RUNNING"}))
}
