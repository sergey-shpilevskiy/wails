package build

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/leaanthony/gosod"
	wailsRuntime "github.com/sergey-shpilevskiy/wails/v2/internal/frontend/runtime"
	"github.com/sergey-shpilevskiy/wails/v2/internal/frontend/runtime/wrapper"

	"github.com/pkg/errors"

	"github.com/leaanthony/slicer"
	"github.com/sergey-shpilevskiy/wails/v2/internal/assetdb"
	"github.com/sergey-shpilevskiy/wails/v2/internal/fs"
	"github.com/sergey-shpilevskiy/wails/v2/internal/html"
	"github.com/sergey-shpilevskiy/wails/v2/internal/project"
	"github.com/sergey-shpilevskiy/wails/v2/internal/shell"
	"github.com/sergey-shpilevskiy/wails/v2/pkg/clilogger"
)

const (
	VERBOSE int = 2
)

// BaseBuilder is the common builder struct
type BaseBuilder struct {
	filesToDelete slicer.StringSlicer
	projectData   *project.Project
	options       *Options
}

// NewBaseBuilder creates a new BaseBuilder
func NewBaseBuilder(options *Options) *BaseBuilder {
	result := &BaseBuilder{
		options: options,
	}
	return result
}

// SetProjectData sets the project data for this builder
func (b *BaseBuilder) SetProjectData(projectData *project.Project) {
	b.projectData = projectData
}

func (b *BaseBuilder) addFileToDelete(filename string) {
	if !b.options.KeepAssets {
		b.filesToDelete.Add(filename)
	}
}

func (b *BaseBuilder) fileExists(path string) bool {
	// if file doesn't exist, ignore
	_, err := os.Stat(path)
	if err != nil {
		return !os.IsNotExist(err)
	}
	return true
}

// buildCustomAssets will iterate through the projects static directory and add all files
// to the application wide asset database.
func (b *BaseBuilder) buildCustomAssets(projectData *project.Project) error {

	// Add trailing slash to Asset directory
	customAssetsDir := filepath.Join(projectData.Path, "assets", "custom") + "/"
	if !b.fileExists(customAssetsDir) {
		err := fs.MkDirs(customAssetsDir)
		if err != nil {
			return err
		}
	}

	assets := assetdb.NewAssetDB()
	err := filepath.Walk(customAssetsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		normalisedPath := filepath.ToSlash(path)
		localPath := strings.TrimPrefix(normalisedPath, customAssetsDir)
		if len(localPath) == 0 {
			return nil
		}
		if data, err := ioutil.ReadFile(filepath.Join(customAssetsDir, localPath)); err == nil {
			assets.AddAsset(localPath, data)
		}

		return nil
	})
	if err != nil {
		return err
	}

	// Write assetdb out to root directory
	assetsDbFilename := fs.RelativePath("../../../assetsdb.go")
	b.addFileToDelete(assetsDbFilename)
	err = ioutil.WriteFile(assetsDbFilename, []byte(assets.Serialize("assets", "wails")), 0644)
	if err != nil {
		return err
	}
	return nil
}

func (b *BaseBuilder) convertFileToIntegerString(filename string) (string, error) {
	rawData, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return b.convertByteSliceToIntegerString(rawData), nil
}

func (b *BaseBuilder) convertByteSliceToIntegerString(data []byte) string {

	// Create string builder
	var result strings.Builder

	if len(data) > 0 {

		// Loop over all but 1 bytes
		for i := 0; i < len(data)-1; i++ {
			result.WriteString(fmt.Sprintf("%v,", data[i]))
		}

		result.WriteString(fmt.Sprintf("%v", data[len(data)-1]))

	}

	return result.String()
}

// CleanUp does post-build housekeeping
func (b *BaseBuilder) CleanUp() {

	// Delete all the files
	b.filesToDelete.Each(func(filename string) {

		// if file doesn't exist, ignore
		if !b.fileExists(filename) {
			return
		}

		// Delete file. We ignore errors because these files will be overwritten
		// by the next build anyway.
		_ = os.Remove(filename)

	})
}

func (b *BaseBuilder) OutputFilename(options *Options) string {
	outputFile := options.OutputFile
	if outputFile == "" {
		target := strings.TrimSuffix(b.projectData.OutputFilename, ".exe")
		if b.projectData.OutputType != "desktop" {
			target += "-" + b.projectData.OutputType
		}
		// If we aren't using the standard compiler, add it to the filename
		if options.Compiler != "go" {
			// Parse the `go version` output. EG: `go version go1.16 windows/amd64`
			stdout, _, err := shell.RunCommand(".", options.Compiler, "version")
			if err != nil {
				return ""
			}
			versionSplit := strings.Split(stdout, " ")
			if len(versionSplit) == 4 {
				target += "-" + versionSplit[2]
			}
		}
		switch b.options.Platform {
		case "windows":
			outputFile = target + ".exe"
		case "darwin", "linux":
			if b.options.Arch == "" {
				b.options.Arch = runtime.GOARCH
			}
			outputFile = fmt.Sprintf("%s-%s-%s", target, b.options.Platform, b.options.Arch)
		}

	}
	return outputFile
}

// CompileProject compiles the project
func (b *BaseBuilder) CompileProject(options *Options) error {

	// Check if the runtime wrapper exists
	err := generateRuntimeWrapper(options)
	if err != nil {
		return err
	}

	verbose := options.Verbosity == VERBOSE
	// Run go mod tidy first
	cmd := exec.Command(options.Compiler, "mod", "tidy")
	cmd.Stderr = os.Stderr
	if verbose {
		println("")
		cmd.Stdout = os.Stdout
	}
	err = cmd.Run()
	if err != nil {
		return err
	}

	// Default go build command
	commands := slicer.String([]string{"build"})

	// Add better debugging flags
	if options.Mode == Dev {
		commands.Add("-gcflags")
		commands.Add(`"all=-N -l"`)
	}

	var tags slicer.StringSlicer
	tags.Add(options.OutputType)
	tags.AddSlice(options.UserTags)

	// Add webview2 strategy if we have it
	if options.WebView2Strategy != "" {
		tags.Add(options.WebView2Strategy)
	}

	if options.Mode == Production {
		tags.Add("production")
	}

	tags.Deduplicate()

	// Add the output type build tag
	commands.Add("-tags")
	commands.Add(tags.Join(","))

	// LDFlags
	ldflags := slicer.String()
	if options.LDFlags != "" {
		ldflags.Add(options.LDFlags)
	}

	if options.Mode == Production {
		ldflags.Add("-w", "-s")
		if runtime.GOOS == "windows" {
			ldflags.Add("-H windowsgui")
		}
	}

	ldflags.Deduplicate()

	if ldflags.Length() > 0 {
		commands.Add("-ldflags")
		commands.Add(ldflags.Join(" "))
	}

	// Get application build directory
	appDir := options.BuildDirectory
	if options.CleanBuildDirectory {
		err = cleanBuildDirectory(options)
		if err != nil {
			return err
		}
	}

	// Set up output filename
	outputFile := b.OutputFilename(options)
	compiledBinary := filepath.Join(appDir, outputFile)
	commands.Add("-o")
	commands.Add(compiledBinary)

	b.projectData.OutputFilename = strings.TrimPrefix(compiledBinary, options.ProjectData.Path)
	options.CompiledBinary = compiledBinary

	// Create the command
	cmd = exec.Command(options.Compiler, commands.AsSlice()...)
	cmd.Stderr = os.Stderr
	if verbose {
		println("  Build command:", commands.Join(" "))
		cmd.Stdout = os.Stdout
	}
	// Set the directory
	cmd.Dir = b.projectData.Path

	// Add CGO flags
	// We use the project/build dir as a temporary place for our generated c headers
	buildBaseDir, err := fs.RelativeToCwd("build")
	if err != nil {
		return err
	}

	cmd.Env = os.Environ() // inherit env

	if options.Platform != "windows" {
		// Use upsertEnv so we don't overwrite user's CGO_CFLAGS
		cmd.Env = upsertEnv(cmd.Env, "CGO_CFLAGS", func(v string) string {
			if v != "" {
				v += " "
			}
			v += "-I" + buildBaseDir
			return v
		})
		// Use upsertEnv so we don't overwrite user's CGO_CXXFLAGS
		cmd.Env = upsertEnv(cmd.Env, "CGO_CXXFLAGS", func(v string) string {
			if v != "" {
				v += " "
			}
			v += "-I" + buildBaseDir
			return v
		})

		cmd.Env = upsertEnv(cmd.Env, "CGO_ENABLED", func(v string) string {
			return "1"
		})
	}

	cmd.Env = upsertEnv(cmd.Env, "GOOS", func(v string) string {
		return options.Platform
	})

	cmd.Env = upsertEnv(cmd.Env, "GOARCH", func(v string) string {
		return options.Arch
	})

	if verbose {
		println("  Environment:", strings.Join(cmd.Env, " "))
	}

	// Run command
	err = cmd.Run()
	cmd.Stderr = os.Stderr

	// Format error if we have one
	if err != nil {
		return err
	}

	if !options.Compress {
		return nil
	}

	fmt.Printf("Compressing application: ")

	// Do we have upx installed?
	if !shell.CommandExists("upx") {
		println("Warning: Cannot compress binary: upx not found")
		return nil
	}

	var args = []string{"--best", "--no-color", "--no-progress", options.CompiledBinary}

	if options.CompressFlags != "" {
		args = strings.Split(options.CompressFlags, " ")
		args = append(args, options.CompiledBinary)
	}

	if verbose {
		println("upx", strings.Join(args, " "))
	}

	output, err := exec.Command("upx", args...).Output()
	if err != nil {
		return errors.Wrap(err, "Error during compression:")
	}
	println("Done.")
	if verbose {
		println(string(output))
	}

	return nil
}

func generateRuntimeWrapper(options *Options) error {
	wrapperDir := filepath.Join(options.WailsJSDir, "wailsjs", "runtime")
	_ = os.RemoveAll(wrapperDir)
	extractor := gosod.New(wrapper.RuntimeWrapper)
	err := extractor.Extract(wrapperDir, nil)
	if err != nil {
		return err
	}

	//ipcdev.js
	err = os.WriteFile(filepath.Join(wrapperDir, "ipcdev.js"), wailsRuntime.DesktopIPC, 0755)
	if err != nil {
		return err
	}
	//runtimedev.js
	err = os.WriteFile(filepath.Join(wrapperDir, "runtimedev.js"), wailsRuntime.RuntimeDesktopJS, 0755)
	if err != nil {
		return err
	}
	return nil
}

// NpmInstall runs "npm install" in the given directory
func (b *BaseBuilder) NpmInstall(sourceDir string, verbose bool) error {
	return b.NpmInstallUsingCommand(sourceDir, "npm install", verbose)
}

// NpmInstallUsingCommand runs the given install command in the specified npm project directory
func (b *BaseBuilder) NpmInstallUsingCommand(sourceDir string, installCommand string, verbose bool) error {

	packageJSON := filepath.Join(sourceDir, "package.json")

	// Check package.json exists
	if !fs.FileExists(packageJSON) {
		// No package.json, no install
		return nil
	}

	install := false

	// Get the MD5 sum of package.json
	packageJSONMD5 := fs.MustMD5File(packageJSON)

	// Check whether we need to npm install
	packageChecksumFile := filepath.Join(sourceDir, "package.json.md5")
	if fs.FileExists(packageChecksumFile) {
		// Compare checksums
		storedChecksum := fs.MustLoadString(packageChecksumFile)
		if storedChecksum != packageJSONMD5 {
			fs.MustWriteString(packageChecksumFile, packageJSONMD5)
			install = true
		}
	} else {
		install = true
		fs.MustWriteString(packageChecksumFile, packageJSONMD5)
	}

	// Install if node_modules doesn't exist
	nodeModulesDir := filepath.Join(sourceDir, "node_modules")
	if !fs.DirExists(nodeModulesDir) {
		install = true
	}

	// check if forced install
	if b.options.ForceBuild {
		install = true
	}

	// Shortcut installation
	if install == false {
		return nil
	}

	// Split up the InstallCommand and execute it
	cmd := strings.Split(installCommand, " ")
	stdout, stderr, err := shell.RunCommand(sourceDir, cmd[0], cmd[1:]...)
	if verbose || err != nil {
		for _, l := range strings.Split(stdout, "\n") {
			fmt.Printf("    %s\n", l)
		}
		for _, l := range strings.Split(stderr, "\n") {
			fmt.Printf("    %s\n", l)
		}
	}

	return err
}

// NpmRun executes the npm target in the provided directory
func (b *BaseBuilder) NpmRun(projectDir, buildTarget string, verbose bool) error {
	stdout, stderr, err := shell.RunCommand(projectDir, "npm", "run", buildTarget)
	if verbose || err != nil {
		for _, l := range strings.Split(stdout, "\n") {
			fmt.Printf("    %s\n", l)
		}
		for _, l := range strings.Split(stderr, "\n") {
			fmt.Printf("    %s\n", l)
		}
	}
	return err
}

// NpmRunWithEnvironment executes the npm target in the provided directory, with the given environment variables
func (b *BaseBuilder) NpmRunWithEnvironment(projectDir, buildTarget string, verbose bool, envvars []string) error {
	cmd := shell.CreateCommand(projectDir, "npm", "run", buildTarget)
	cmd.Env = append(os.Environ(), envvars...)
	var stdo, stde bytes.Buffer
	cmd.Stdout = &stdo
	cmd.Stderr = &stde
	err := cmd.Run()
	if verbose || err != nil {
		for _, l := range strings.Split(stdo.String(), "\n") {
			fmt.Printf("    %s\n", l)
		}
		for _, l := range strings.Split(stde.String(), "\n") {
			fmt.Printf("    %s\n", l)
		}
	}
	return err
}

// BuildFrontend executes the `npm build` command for the frontend directory
func (b *BaseBuilder) BuildFrontend(outputLogger *clilogger.CLILogger) error {

	verbose := b.options.Verbosity == VERBOSE

	frontendDir := filepath.Join(b.projectData.Path, "frontend")

	// Check there is an 'InstallCommand' provided in wails.json
	if b.projectData.InstallCommand == "" {
		// No - don't install
		outputLogger.Println("No Install command. Skipping.")
	} else {
		// Do install if needed
		outputLogger.Print("Installing frontend dependencies: ")
		if verbose {
			outputLogger.Println("")
			outputLogger.Println("  Install command: '" + b.projectData.InstallCommand + "'")
		}
		if err := b.NpmInstallUsingCommand(frontendDir, b.projectData.InstallCommand, verbose); err != nil {
			return err
		}
		outputLogger.Println("Done.")
	}

	// Check if there is a build command
	var buildCommand string
	switch b.projectData.OutputType {
	case "dev":
		buildCommand = b.projectData.DevCommand
	default:
		buildCommand = b.projectData.BuildCommand
	}
	if buildCommand == "" {
		outputLogger.Println("No Build command. Skipping.")
		// No - ignore
		return nil
	}

	outputLogger.Print("Compiling frontend: ")
	cmd := strings.Split(buildCommand, " ")
	if verbose {
		outputLogger.Println("")
		outputLogger.Println("  Build command: '" + buildCommand + "'")
	}
	stdout, stderr, err := shell.RunCommand(frontendDir, cmd[0], cmd[1:]...)
	if verbose || err != nil {
		for _, l := range strings.Split(stdout, "\n") {
			fmt.Printf("    %s\n", l)
		}
		for _, l := range strings.Split(stderr, "\n") {
			fmt.Printf("    %s\n", l)
		}
	}
	if err != nil {
		return err
	}

	outputLogger.Println("Done.")
	return nil
}

// ExtractAssets gets the assets from the index.html file
func (b *BaseBuilder) ExtractAssets() (*html.AssetBundle, error) {

	// Read in html
	//return html.NewAssetBundle(b.projectData.HTML)
	return nil, nil
}

func upsertEnv(env []string, key string, update func(v string) string) []string {
	newEnv := make([]string, len(env), len(env)+1)
	found := false
	for i := range env {
		if strings.HasPrefix(env[i], key+"=") {
			eqIndex := strings.Index(env[i], "=")
			val := env[i][eqIndex+1:]
			newEnv[i] = fmt.Sprintf("%s=%v", key, update(val))
			found = true
			continue
		}
		newEnv[i] = env[i]
	}
	if !found {
		newEnv = append(newEnv, fmt.Sprintf("%s=%v", key, update("")))
	}
	return newEnv
}
