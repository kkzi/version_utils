package main

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	Version   string `json:"version"`
	Compiler  string `json:"compiler"`
	Output    string `json:"output"`
	Publisher string `json:"publisher"`
	Url       string `json:"url"`
	Apps      []App  `json:"apps"`
}

type App struct {
	Id          string       `json:"app_id"`
	Name        string       `json:"app_name"`
	ExeName     string       `json:"app_exe"`
	BuildPath   string       `json:"build_path"`
	SetupTarget string       `json:"setup_target_path"`
	VcRedist    string       `json:"vcredist"`
	Externs     []ExternPath `json:"extern_path"`
	WorkPath    string
}

type ExternPath struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Override bool   `json:"override"`
}

var log zerolog.Logger
var ignores []string
var innoProfileTemplate = `
; Script generated by the Inno Setup Script Wizard.
; SEE THE DOCUMENTATION FOR DETAILS ON CREATING INNO SETUP SCRIPT FILES!

$DEFINES$

[Setup]
; NOTE: The value of AppId uniquely identifies this application.
; Do not use the same AppId value in installers for other applications.
; (To generate a new GUID, click Tools | Generate GUID inside the IDE.)
AppId=$APPID$
AppName={#AppName}
AppVersion={#AppVersion}
;AppVerName={#AppName} {#AppVersion}
AppPublisher={#AppPublisher}
AppPublisherURL={#AppURL}
AppSupportURL={#AppURL}
AppUpdatesURL={#AppURL}
DefaultDirName=$SETUP_TARGET$
DefaultGroupName=Atom
OutputBaseFilename={#AppName}_{#AppVersion}
Compression=lzma
SolidCompression=yes
PrivilegesRequired=admin

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
; Program Files
$PROGRAM_FILES$

; Extern Files
$EXTERN_FILES$

; NOTE: Don't use "Flags: ignoreversion" on any shared system files

; can not replace \ to /
[Icons]
Name: "{group}\{#AppName}"; Filename: "{app}\{#AppExeName}"
Name: "{commondesktop}\{#AppName}"; Filename: "{app}\{#AppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#AppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(AppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
$RUN$

; copy a vcredist_x64.exe to install path first
[Code]
$CODE$
`

var config = Config{}

func loadConfig(cfg string) {
	log.Info().Str("path", cfg).Msg("load config")
	text, err := ioutil.ReadFile(cfg)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	if err := json.Unmarshal(text, &config); err != nil {
		log.Fatal().Err(err).Msg("")
	}
	config.Output = filepath.ToSlash(config.Output)

	for idx, app := range config.Apps {
		if !filepath.IsAbs(app.BuildPath) {
			config.Apps[idx].BuildPath, err = filepath.Abs(app.BuildPath)
			if err != nil {
				log.Fatal().Err(err)
			}
		}
		config.Apps[idx].BuildPath = filepath.ToSlash(config.Apps[idx].BuildPath)
		log.Info().Str("build path", config.Apps[idx].BuildPath).Send()

		config.Apps[idx].WorkPath = filepath.ToSlash(config.Output + "/" + app.Name)
		log.Info().Str("work path", config.Apps[idx].WorkPath).Send()

		if !filepath.IsAbs(app.SetupTarget) {
			log.Fatal().Str("setup path", app.SetupTarget).Msg("Invalid setup_target_path")
		}

		for i, ext := range app.Externs {
			if !filepath.IsAbs(ext.Source) {
				path, err := filepath.Abs(ext.Source)
				if err != nil {
					log.Fatal().Err(err).Msg("")
				}
				config.Apps[idx].Externs[i].Source = filepath.ToSlash(path)
			}
		}
	}
	log.Info().Str("path", cfg).Msg("load config ok")
}

func loadIgnore() {
	log.Info().Msg("load ignore list")
	text, err := ioutil.ReadFile("ignore.txt")
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	for _, i := range strings.Split(string(text), "\n") {
		fixed := strings.TrimSpace(i)
		if len(fixed) > 0 {
			ignores = append(ignores, fixed)
		}
	}
	log.Info().Strs("ignore", ignores).Msg("load ignore list ok")
}

func createVersionConfig(app App) {
	version := config.Version
	if strings.HasSuffix(version, ".h") {
		txt, err := ioutil.ReadFile(version)
		if err != nil {
			log.Info().Str("read version file failed. ", version).Msg("")
			return
		}
		lines := strings.Split(string(txt), "\n")
		var major, minor, patch, build, ref string
		for _, l := range lines {
			if !strings.Contains(l, "=") || !strings.Contains(l, ";") {
				continue
			}
			token := strings.Split(l, "=")[1]
			token = strings.Split(token, ";")[0]
			token = strings.Trim(token, " ")
			if strings.Contains(l, "major") {
				major = token
			} else if strings.Contains(l, "minor") {
				minor = token
			} else if strings.Contains(l, "patch") {
				patch = token
			} else if strings.Contains(l, "build") {
				build = strings.Replace(token, `"`, "", -1)
			} else if strings.Contains(l, "ref") {
				ref = strings.Replace(token, `"`, "", -1)
			}
		}
		config.Version = fmt.Sprintf("%s.%s.%s.%s.%s", major, minor, patch, build, ref)
		log.Info().Str("app version", config.Version).Msg("")
	} else {
		_ = ioutil.WriteFile(filepath.Join(app.WorkPath, "version"), []byte(version), os.ModePerm)
		log.Info().Msg("create version file")
	}
}

func copyFile(from, to string) {
	stat, err := os.Stat(from)
	if err != nil || stat.IsDir() {
		return
	}
	from = filepath.ToSlash(from)
	to = filepath.ToSlash(to)
	src, _ := os.Open(from)
	defer src.Close()

	dstdir := filepath.Dir(to)
	dir, err := os.Stat(dstdir)
	if dir == nil || os.IsNotExist(err) {
		_ = os.MkdirAll(dstdir, os.ModePerm)
	}

	dst, _ := os.Create(to)
	if dst == nil {
		log.Fatal().Str("from", from).Str("to", to).Err(err).Msg("")
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		log.Fatal().Str("from", from).Str("to", to).Err(err).Msg("")
	}
}

func copyNewFiles(app App) {
	log.Info().Str("path", app.WorkPath).Msg("remove dir")
	_ = os.RemoveAll(app.WorkPath)

	log.Info().Str("path", app.WorkPath).Msg("create dir")
	err := os.MkdirAll(app.WorkPath, os.ModePerm)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}
	log.Info().Str("from", app.BuildPath).Str("to", app.WorkPath).Msg("copy files")

	_ = filepath.Walk(app.BuildPath, func(path string, info os.FileInfo, err error) error {
		stat, _ := os.Stat(path)
		if stat.IsDir() {
			return nil
		}
		if needIgnore(path) {
			return nil
		}

		relativePath := strings.Replace(filepath.ToSlash(path), filepath.ToSlash(app.BuildPath), "", -1)
		dest := filepath.Join(app.WorkPath, relativePath)
		copyFile(path, dest)
		return nil
	})

	if len(app.VcRedist) > 0 {
		vc := "vcredist_x64.exe"
		vcfile, _ := os.Stat(vc)
		if vcfile != nil {
			copyFile(vc, app.WorkPath+"/"+vc)
		} else {
			app.VcRedist = ""
			log.Warn().Str("file", vc).Msg("can not find file")
		}
	}

	log.Info().Msg("copy files ok")
}

func generateInnoProfile(app App) string {
	var defines, programs, externs /*, programs*/ []string
	defines = append(defines, `#define AppName "`+app.Name+`"`)
	defines = append(defines, `#define AppVersion "`+config.Version+`"`)
	defines = append(defines, `#define AppPublisher "`+config.Publisher+`"`)
	defines = append(defines, `#define AppURL "`+config.Url+`"`)
	defines = append(defines, `#define AppExeName "`+app.ExeName+`"`)
	defines = append(defines, `#define AppSourceDir "`+app.WorkPath+`"`)

	if app.WorkPath != "" {
		flags := "ignoreversion recursesubdirs createallsubdirs"
		text := fmt.Sprintf(`Source: "{#AppSourceDir}/*"; Excludes: "\config"; DestDir: "{app}/"; Flags: %s`, flags)
		programs = append(programs, text)
	}

	for _, e := range app.Externs {
		flags := "ignoreversion recursesubdirs createallsubdirs"
		if !e.Override {
			flags += " onlyifdoesntexist uninsneveruninstall"
		}
		externs = append(externs, `Source: "`+e.Source+`"; DestDir: "`+e.Target+`"; Flags: `+flags)
	}

	profile := innoProfileTemplate
	profile = strings.Replace(profile, "$DEFINES$", strings.Join(defines, "\n"), -1)
	profile = strings.Replace(profile, "$APPID$", app.Id, -1)
	profile = strings.Replace(profile, "$SETUP_TARGET$", app.SetupTarget, -1)
	profile = strings.Replace(profile, "$PROGRAM_FILES$", strings.Join(programs, "\n"), -1)
	profile = strings.Replace(profile, "$EXTERN_FILES$", strings.Join(externs, "\n"), -1)

	if len(app.VcRedist) > 0 {
		run := `Filename: "{app}/vcredist_x64.exe"; Parameters:/q;WorkingDir:{tmp};Flags:skipifdoesntexist;StatusMsg:"Installing Runtime...";Check:NeedInstallVCRuntime`
		code := `
var NeedVcRuntime: Boolean;
 
function NeedInstallVCRuntime(): Boolean;
begin
  Result := NeedVcRuntime;
end;
 
function InitializeSetup(): Boolean;
var version: Cardinal;
begin
  if RegQueryDWordValue(HKLM, 'SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\$(VCREGKEY)$', 'Version', version) = false then begin
    NeedVcRuntime := true;
  end;
  Result := true;
end;
`
		code = strings.Replace(code, "$VCREGKEY$", app.VcRedist, -1)
		profile = strings.Replace(profile, "$RUN$", run, -1)
		profile = strings.Replace(profile, "$CODE$", code, -1)
	} else {
		profile = strings.Replace(profile, "$RUN$", "", -1)
		profile = strings.Replace(profile, "$CODE$", "", -1)
	}

	iss := filepath.Join(config.Output, app.Name+"_"+config.Version+".iss")
	iss = filepath.ToSlash(iss)
	log.Info().Str("path", iss).Msg("create iss file")
	if err := ioutil.WriteFile(iss, []byte(profile), os.ModePerm); err != nil {
		log.Fatal().Err(err).Msg("create iss file failed")
	}
	log.Info().Str("path", iss).Msg("create iss file ok")
	return iss
}

func generateSetupFile(iss string) {
	log.Info().Str("config file", iss).Msg("create setup file")
	cmd := exec.Command(config.Compiler, "/O"+config.Output, iss)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal().Err(err).Msg(string(output))
	}
	log.Info().Msg("create setup file ok")
}

func needIgnore(path string) bool {
	path = filepath.ToSlash(path)
	for _, i := range ignores {
		if strings.HasSuffix(path, i) {
			return true
		}
		matched, _ := filepath.Match(i, path)
		if matched {
			return true
		}
	}
	return false
}

func main() {
	fmt.Println("======= version releaser 2.2 build.20201111 =======")
	var cw = zerolog.ConsoleWriter{Out: os.Stdout}
	//cw.FormatTimestamp = func(i interface{}) string {
	//	return fmt.Sprintf("%s", i)
	//}
	log = zerolog.New(cw).With().Timestamp().Logger()

	cfg := "config.json"
	if len(os.Args) > 1 {
		cfg = os.Args[1]
	}
	loadConfig(cfg)
	loadIgnore()

	var iss []string
	for _, app := range config.Apps {
		copyNewFiles(app)
		createVersionConfig(app)
		iss = append(iss, generateInnoProfile(app))
	}
	for _, i := range iss {
		generateSetupFile(i)
	}
}
