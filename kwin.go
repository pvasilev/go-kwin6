// Package go_kwin6 implements utility routines for interacting with KWin composited window manager
package go_kwin6

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

/*
Experimental KWin6 scripting for getting screens, desktops and windows and moving the windows around
Have this variable set up and recognized by the system/journal:
export QT_LOGGING_RULES="kwin_*.debug=true"
KWin scripts are saved in temp folder as files, then loaded in KWin scripting machine, executed and deregistered and the script file deleted
*/

const (
	dbusSend   = "/usr/bin/dbus-send"
	journalCtl = "/usr/bin/journalctl"
)

type (
	// KWin is a common methods receiver to act like an object
	KWin struct{}
	// Point is a struct that contains integer valued coordinates for screen geometry
	Point struct {
		X int `json:"x"`
		Y int `json:"y"`
	}
	// Rect is a struct that contains integer valued points for screen geometry
	Rect struct {
		TopLeft     Point `json:"topLeft"`
		BottomRight Point `json:"bottomRight"`
	}
	// Screen is a struct that contains the main properties of the corresponding KWin::Output object which represents a
	//physical screen/monitor
	Screen struct {
		Name         string  `json:"name"`
		Geometry     Rect    `json:"geometry"`
		Manufacturer string  `json:"manufacturer"`
		Model        string  `json:"model"`
		SerialNumber string  `json:"serial"`
		PixelRatio   float64 `json:"pixelRatio"`
	}
	// Desktop is a struct that contains the main properties of KWin::VirtualDesktop object which represents a virtual
	//desktop containing client program windows
	Desktop struct {
		Id        string `json:"id"`
		Index     int    `json:"index"`
		Name      string `json:"name"`
		X11Number int    `json:"x11Number"`
	}
	// Window is a struct that contains the most useful properties of KWin::Window object which represents a client
	//program window
	Window struct {
		Id               string      `json:"id"`
		Caption          string      `json:"caption"`
		Pid              int         `json:"pid"`
		CmdLine          string      `json:"cmdline"`
		AppName          string      `json:"appname"`
		X                float64     `json:"x"`
		Y                float64     `json:"y"`
		Width            float64     `json:"width"`
		Height           float64     `json:"height"`
		Fullscreen       bool        `json:"fullscreen"`
		OnAllDesktops    bool        `json:"onAllDesktops"`
		KeepAbove        bool        `json:"keepAbove"`
		KeepBelow        bool        `json:"keepBelow"`
		Minimized        bool        `json:"minimized"`
		DesktopIds       []uuid.UUID `json:"desktopIds"`
		Desktops         []Desktop   `json:"desktops"`
		DemandsAttention bool        `json:"demandsAttention"`
	}
	// Environment is a struct that contains all detected Screen, virtual Desktop and Window objects on the system
	Environment struct {
		// Screens is a map of Screen objects, where the key is the Screen name
		Screens map[string]Screen `json:"screens"`
		// Desktops is a map of Desktop objects, where the key is the Desktop uuid
		Desktops map[uuid.UUID]Desktop `json:"desktops"`
		// Windows is a map of Window objects, where the key is the Window uuid
		Windows map[uuid.UUID]Window `json:"windows"`
	}
)

// NewKWin is a helper method which creates new instance of the KWin struct
func NewKWin() KWin {
	return KWin{}
}

// callProgramAndReadOutput - starts a process for a given command and arguments, waits for it to finish and reads the
// process output
func (k KWin) callProgramAndReadOutput(command string, args ...string) ([]string, error) {
	cmd := exec.Command(command, args...)
	if cmd.Err != nil {
		return nil, cmd.Err
	}
	stdout, err := cmd.StdoutPipe()
	errout, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	processOutput := make([]string, 0)
	stdScanner := bufio.NewScanner(stdout)
	for stdScanner.Scan() {
		processOutput = append(processOutput, stdScanner.Text())
	}
	errScanner := bufio.NewScanner(errout)
	for errScanner.Scan() {
		processOutput = append(processOutput, errScanner.Text())
	}

	if err := cmd.Wait(); err != nil {
		fmt.Printf("Command finished with error: %v\n", err)
		for i := range processOutput {
			fmt.Printf("%s\n", processOutput[i])
		}
		return nil, err
	}

	return processOutput, nil
}

// callDbusSend is a helper function which calls dbus-send command with the given parameters and returns the process
// output
func (k KWin) callDbusSend(args ...string) ([]string, error) {
	return k.callProgramAndReadOutput(dbusSend, args...)
}

// loadScript calls KWin scripting infrastructure to load a file which contains a JavaScript scriptlet and returns the
// script registration number inside KWin, with which it can be later invoked/stopped
func (k KWin) loadScript(scriptPath string) (int, error) {
	output, err := k.callDbusSend(
		"--print-reply",
		"--dest=org.kde.KWin",
		"/Scripting", "org.kde.kwin.Scripting.loadScript", "string:"+scriptPath)
	if err != nil {
		return -1, err
	}
	if len(output) != 2 {
		return -1, fmt.Errorf("script load failed: %s", output)
	}
	sa := strings.Fields(output[1])
	if len(sa) != 2 {
		return -1, fmt.Errorf("script load failed: %s", output)
	}
	sRegNo := sa[1]
	iRegNo, err := strconv.Atoi(sRegNo)
	if err != nil {
		return -1, err
	}
	return iRegNo, nil
}

// runScript calls KWin scripting infrastructure to execute a previously loaded JavaScript scriptlet. It returns error
// on failure, the actual script generated output is gathered by journalctl
func (k KWin) runScript(scriptNo int) error {
	_, err := k.callDbusSend(
		"--print-reply",
		"--dest=org.kde.KWin",
		fmt.Sprintf("/Scripting/Script%d", scriptNo), "org.kde.kwin.Script.run")

	if err != nil {
		return err
	}
	return nil
}

// stopScript calls KWin scripting infrastructure to stop and deregister a previously loaded JavaScript scriptlet.
// It returns error on failure
func (k KWin) stopScript(scriptNo int) error {
	_, err := k.callDbusSend("--print-reply", "--dest=org.kde.KWin", fmt.Sprintf("/Scripting/Script%d", scriptNo), "org.kde.kwin.Script.stop")

	if err != nil {
		return err
	}
	return nil
}

// getJournal executes the journalctl to gather the previously executed script output, found between the two timestamps
// and filtered by the QT_ flags below
func (k KWin) getJournal(from, to time.Time) ([]string, error) {
	format := "2006-01-02 15:04:05.000000"
	since := from.Format(format)
	until := to.Format(format)
	output, err := k.callProgramAndReadOutput(
		journalCtl,
		"QT_CATEGORY=js", "QT_CATEGORY=kwin_scripting",
		"-o", "cat",
		"--since", since,
		"--until", until,
		"--no-pager")
	if err != nil {
		return nil, err
	}
	return output, nil
}

// loadExecuteAndGetOutput executes given JavaScript code by
//
//	Saving into a temporary file
//	Loading/Registering it with KWin scripting infrastructure
//	Running the script
//	Stopping the script
//	Gathering the script output from the journal for the time window the script was running
func (k KWin) loadExecuteAndGetOutput(script string) ([]string, error) {
	scriptFile, err := os.CreateTemp(os.TempDir(), "kwin_script_*.js")
	if err != nil {
		return nil, err
	}
	defer func() {
		err := scriptFile.Close()
		if err != nil {
			fmt.Printf("Error closing script file: %v\n", err)
			return
		}
		err = os.Remove(scriptFile.Name())
		if err != nil {
			fmt.Printf("Error removing script file: %v\n", err)
			return
		}
	}()
	_, err = scriptFile.WriteString(script)
	if err != nil {
		fmt.Printf("Error writing script file: %v\n", err)
		return nil, err
	}
	err = os.Chmod(scriptFile.Name(), 0777) //KWin needs to be able to read the script, 777 may be a bit excessive
	if err != nil {
		fmt.Printf("Error chmod: %v\n", err)
		return nil, err
	}

	scriptNo, err := k.loadScript(scriptFile.Name())
	if err != nil {
		fmt.Printf("Error loading script: %v\n", err)
		return nil, err
	}

	startTime := time.Now()
	err = k.runScript(scriptNo)
	if err != nil {
		fmt.Printf("Error running script: %v\n", err)
		return nil, err
	}

	err = k.stopScript(scriptNo)
	endTime := time.Now()
	if err != nil {
		fmt.Printf("Error stopping script: %v\n", err)
		return nil, err
	}

	journalOutput, err := k.getJournal(startTime, endTime)
	if err != nil {
		fmt.Printf("Error getting journal output: %v\n", err)
		return nil, err
	}
	return journalOutput, nil
}

// getProcessCmdLine uses the linux /proc infrastructure to get a process command line by given PID
func (k KWin) getProcessCmdLine(processId int) (string, error) {
	proc := fmt.Sprintf("/proc/%d/cmdline", processId)
	cmdLine, err := os.ReadFile(proc)
	if err != nil {
		fmt.Printf("Error reading process command line: %v\n", err)
		return "", err
	}
	cmdLine = bytes.ReplaceAll(cmdLine, []byte("\x00"), []byte("\x20"))
	return strings.TrimSpace(string(cmdLine)), nil
}

// GetScreens returns a map of detected Screen objects where the map key is the Screen name
func (k KWin) GetScreens() (map[string]Screen, error) {
	script := `
	for (var i = 0; i< workspace.screens.length; i++) {
		var screen = workspace.screens[i]
		var out = "{"
		out += "\"name\": \""+screen.name+"\","
		out += "\"manufacturer\": \""+screen.manufacturer+"\","
		out += "\"model\": \""+screen.model+"\","
		out += "\"serial\": \""+screen.serialNumber+"\","
		out += "\"pixelRatio\": "+screen.devicePixelRatio+","
		out += "\"geometry\": {"
		out += "\"topLeft\": {"
		out += "\"x\":"+screen.geometry.left+","
		out += "\"y\":"+screen.geometry.top
		out += "},"
		out += "\"bottomRight\": {"
		out += "\"x\":"+screen.geometry.right+","
		out += "\"y\":"+screen.geometry.bottom
		out += "}"
		out += "}"
		out += "}"
		print(out)
	}`
	output, err := k.loadExecuteAndGetOutput(script)
	if err != nil {
		fmt.Printf("Error running script for screens list: %v\n", err)
		return nil, err
	}
	outputMap := make(map[string]Screen)
	for _, s := range output {
		d := Screen{}
		ss := strings.ReplaceAll(s, "js: ", "")
		if err := json.Unmarshal([]byte(ss), &d); err != nil {
			return nil, err
		}
		outputMap[d.Name] = d
	}
	return outputMap, nil
}

// GetDesktops returns a map of detected Desktop objects where the map key is the Desktop ID
func (k KWin) GetDesktops() (map[uuid.UUID]Desktop, error) {
	script := `
	for (var i = 0; i < workspace.desktops.length; i++) {
		var desktop = workspace.desktops[i]
		var out = "{"
		out += "\"id\": \""+desktop.id+"\","
		out += "\"index\": "+i+","
		out += "\"name\": \""+desktop.name+"\","
		out += "\"x11Number\": "+desktop.x11DesktopNumber
		out += "}"
		print(out)
	}`
	output, err := k.loadExecuteAndGetOutput(script)
	if err != nil {
		fmt.Printf("Error running script for desktops list: %v\n", err)
		return nil, err
	}
	outputMap := make(map[uuid.UUID]Desktop)
	for _, s := range output {
		d := Desktop{}
		ss := strings.ReplaceAll(s, "js: ", "")
		if err := json.Unmarshal([]byte(ss), &d); err != nil {
			return nil, err
		}
		outputMap[uuid.MustParse(d.Id)] = d
	}
	return outputMap, nil
}

// GetWindows returns a map of detected Window objects where the map key is the Window ID
func (k KWin) GetWindows(desktops map[uuid.UUID]Desktop) (map[uuid.UUID]Window, error) {
	script := `
	for (const window of workspace.windowList()) {
		if (window.specialWindow) {
			continue;
		}
		var out = "{"
		out += "\"id\": \""+window.internalId.toString().replace(/{/, "").replace(/}/, "")+"\","
		out += "\"caption\": \""+window.caption.replace(/\"/g, "")+"\","
		out += "\"pid\": "+window.pid+","
		out += "\"x\": "+window.x+","
		out += "\"y\": "+window.y+","
		out += "\"width\": "+window.width+","
		out += "\"height\": "+window.height+","
		out += "\"fullScreen\": "+window.fullScreen+","
		out += "\"onAllDesktops\": "+window.onAllDesktops+","
		out += "\"keepAbove\": "+window.keepAbove+","
		out += "\"keepBelow\": "+window.keepBelow+","
		out += "\"minimized\": "+window.minimized+","
    	out += "\"demandsAttention\": "+window.demandsAttention+","
        out += "\"desktopIds\": ["
        for (var i = 0; i < window.desktops.length; i++) {
            d = window.desktops[i];
            out += "\""+d.id+"\"";
            if (i < window.desktops.length-1) {
                out += ","
            }
        }
        out += "]"
		out += "}"
		print(out)
	}`
	output, err := k.loadExecuteAndGetOutput(script)
	if err != nil {
		fmt.Printf("Error running script for windows list: %v\n", err)
		return nil, err
	}
	outputMap := make(map[uuid.UUID]Window)
	for _, s := range output {
		d := Window{}
		ss := strings.ReplaceAll(s, "js: ", "")
		if err := json.Unmarshal([]byte(ss), &d); err != nil {
			return nil, err
		}
		rawCmdLine, err := k.getProcessCmdLine(d.Pid)
		if err != nil {
			fmt.Printf("Can't process windows list: %v\n", err)
			return nil, err
		}
		cmdLine := strings.Fields(rawCmdLine)[0]
		d.CmdLine = cmdLine
		saCmdLine := strings.Split(cmdLine, "/")
		appName := strings.TrimSpace(saCmdLine[len(saCmdLine)-1])
		d.AppName = appName
		if desktops != nil {
			d.Desktops = make([]Desktop, len(d.DesktopIds))
			for i := range d.DesktopIds {
				d.Desktops[i] = desktops[d.DesktopIds[i]]
			}
		}
		outputMap[uuid.MustParse(d.Id)] = d
	}
	return outputMap, nil
}

// GetEnvironment is a helper method, which gathers all available Screen, Desktop and Window information and returns it
// as a single structure
func (k KWin) GetEnvironment() (Environment, error) {
	screens, err := k.GetScreens()
	if err != nil {
		fmt.Printf("Error getting screens: %v\n", err)
		return Environment{}, err
	}
	desktops, err := k.GetDesktops()
	if err != nil {
		fmt.Printf("Error getting desktops: %v\n", err)
		return Environment{}, err
	}
	windows, err := k.GetWindows(desktops)
	if err != nil {
		fmt.Printf("Error getting windows: %v\n", err)
		return Environment{}, err
	}
	return Environment{
		Screens:  screens,
		Desktops: desktops,
		Windows:  windows,
	}, nil
}

// MoveWindowToDesktop will attempt to move a given Window to a given Desktop
func (k KWin) MoveWindowToDesktop(w Window, d Desktop) error {
	return k.MoveWindowToDesktops(w, []Desktop{d})
}

// MoveWindowToDesktops will attempt to move a given Window to a given array of multiple Desktop's
//
//	NOTE: This only works on Wayland. On X11 the window will be moved to the last Desktop in the list
func (k KWin) MoveWindowToDesktops(w Window, ds []Desktop) error {
	script := `
    targetDesktopIds = %s;
	windowId = "%s";
    var d = [];
    for (const desktop of workspace.desktops) {
        if (targetDesktopIds.includes(desktop.id)) {
            d.push(desktop);
        }
    }
    if (d && d.length > 0) {
        var w = undefined;
        for (const window of workspace.windowList()) {
            wid = window.internalId.toString().replace(/{/, "").replace(/}/, "");
            if (wid === windowId) {
                w = window;
                break;
            }
        }
        if (w && w.moveable) {
            w.desktops = d;
        }
    }`
	targetDesktops := "["
	for i, d := range ds {
		targetDesktops += "\"" + d.Id + "\""
		if i < len(ds)-1 {
			targetDesktops += ","
		}
	}
	targetDesktops += "]"
	_, err := k.loadExecuteAndGetOutput(fmt.Sprintf(script, targetDesktops, w.Id))
	return err
}

// MoveWindowToScreen will attempt to move a given Window to a given Screen output
func (k KWin) MoveWindowToScreen(w Window, s Screen) error {
	script := `
    targetScreenName = "%s"
    windowId = "%s";
    
    var s = undefined;
    for (const screen of workspace.screens) {
        if (screen.name === targetScreenName) {
            s = screen;
            break;
        }
    }
    if (s) {
        var w = undefined;
        for (const window of workspace.windowList()) {
            wid = window.internalId.toString().replace(/{/, "").replace(/}/, "");
            if (wid === windowId) {
                w = window;
                break;
            }
        }
        if (w && w.moveable) {
            workspace.sendClientToScreen(w, s);
        }
    }`

	output, err := k.loadExecuteAndGetOutput(fmt.Sprintf(script, s.Name, w.Id))
	for _, s := range output {
		fmt.Println(s)
	}

	return err
}

// MoveWindowToDesktopsAndScreen will attempt to move a given Window to a given list of Desktop's and to a given Screen
// output
func (k KWin) MoveWindowToDesktopsAndScreen(w Window, ds []Desktop, s Screen) error {
	err := k.MoveWindowToDesktops(w, ds)
	if err != nil {
		return err
	}
	return k.MoveWindowToScreen(w, s)
}

// MaximizeWindow will attempt to maximize window both horizontally and vertically
func (k KWin) MaximizeWindow(w Window) error {
	return k.maximizeWindowHV(w, true, true)
}

// MaximizeWindowHorizontally will attempt to maximize window horizontally
func (k KWin) MaximizeWindowHorizontally(w Window) error {
	return k.maximizeWindowHV(w, true, false)
}

// MaximizeWindowVertically will attempt to maximize window vertically
func (k KWin) MaximizeWindowVertically(w Window) error {
	return k.maximizeWindowHV(w, false, true)
}

func (k KWin) maximizeWindowHV(w Window, maximizeHorizontally, maximizeVertically bool) error {
	script := `
    windowId = "%s";
    maximizeHorizontally = %v;
    maximizeVertically = %v;
    for (const window of workspace.windowList()) {
        wid = window.internalId.toString().replace(/{/, "").replace(/}/, "");
        if (wid === windowId) {
            window.setMaximize(maximizeVertically, maximizeHorizontally);
            break;
        }
    }`
	command := fmt.Sprintf(script, w.Id, maximizeHorizontally, maximizeVertically)
	output, err := k.loadExecuteAndGetOutput(command)
	for _, s := range output {
		fmt.Println(s)
	}

	return err
}

// MinimizeWindow will attempt to minimize window
func (k KWin) MinimizeWindow(w Window) error {
	script := `
    windowId = "%s";
    for (const window of workspace.windowList()) {
        wid = window.internalId.toString().replace(/{/, "").replace(/}/, "");
        if (wid === windowId) {
            window.minimized = true;
            break;
        }
    }`
	command := fmt.Sprintf(script, w.Id)
	output, err := k.loadExecuteAndGetOutput(command)
	for _, s := range output {
		fmt.Println(s)
	}

	return err
}

// SetWindowDemandsAttention will attempt to set the window state of demanding user attention to the specified value
func (k KWin) SetWindowDemandsAttention(w Window, demandsAttention bool) error {
	script := `
		windowId = "%s";
		for (const window of workspace.windowList()) {
			var w = undefined;
			for (const window of workspace.windowList()) {
				wid = window.internalId.toString().replace(/{/, "").replace(/}/, "");
				if (wid === windowId) {
					w = window;
					break;
				}
			}
			if (w) {
				w.demandsAttention = %s;
			}
		}`
	command := fmt.Sprintf(script, w.Id, demandsAttention)
	output, err := k.loadExecuteAndGetOutput(command)
	for _, s := range output {
		fmt.Println(s)
	}

	return err
}

// WindowDemandAttention will attempt to set the given window to demand user attention
func (k KWin) WindowDemandAttention(w Window) error {
	return k.SetWindowDemandsAttention(w, true)
}

// WindowUnDemandAttention will attempt to set the given window to not demand user attention even if it currently does
func (k KWin) WindowUnDemandAttention(w Window) error {
	return k.SetWindowDemandsAttention(w, false)
}
