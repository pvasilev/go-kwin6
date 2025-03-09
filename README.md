# go-kwin6
Golang **KWin6** interfacing example for enumerating and moving Plasma/Wayland windows around [^x11]


This project is an experiment in trying to interface the **KWin** window compositor with the goal of listing the registered 
Screens, Desktops and Windows and manipulating the Windows placement/geometry.

My OS is Arch Linux, running KDE Plasma 6 on Wayland graphics platform.

I have a case where on my home/work machine I have 4 screens and some 20 virtual desktops, each one for a specific activity, with 
relevant applications loaded and open there. Currently, Wayland does not support proper session storing/restoring, so each
time I reboot the computer I need to manually start some 50-ish applications and move them to their relevant virtual 
desktop and screen. I wanted to automate this process by automatically launching them all at startup and then moving 
them to their places. This code deals with the "moving" activity.

I tried searching the internets for ready solutions and I didn't seem to find any, so I decided to make my own. After a 
bit of fiddling with **wmctrl** - I realized I can't make it work for Wayland, and it seemed like the only option is to 
utilize **KWin** scripting infrastructure. KWin is capable of loading JavaScript scriptlets from a file, registering them 
under a consecutive number, and then they can be executed (or deregistered) using this reference number. If the script 
needs to produce any output - it does so in the system journal and can be gathered using **journalctl** by specifyng 
filters on time window in which the script was running and a specific **QT_** debug flags. In order for this to work, the 
following environment variable needs to be set:

```bash
export QT_LOGGING_RULES="kwin_*.debug=true"
```


The way this library works is it generates a dynamic JavaScript scriptlet for each activity I want KWin to execute. 
This is the workflow:
1. Generate the JavaScript scriptlet as a string
2. Store the scriptlet in a temp file, accessible by KWin
3. Load and register it in KWin, returning the registration ID (usually next available scriptlet number in KWin)
4. Call KWin to run the registered scriptlet using its registration number
5. Call KWin to deregister the scriptlet, thus releasing the scriptlet number
6. Delete the JavaScript scriptlet file
7. Gather all journalctl information during the time window in which the scriptlet was running

The scriptlets are written in a manner, which generates JSON strings as an output in the journal, for easy **Go** struct 
demarshalling. I tried to keep them in their relevant methods so that whoever wants to reuse them in different language 
or even manually, can just copy/paste/extract them and put them in their code.

To test the scripts themselves, I have written a bash script that loads, runs and deregisters a sample scriptlet and 
returns the collected journalctl information. In this case I have left the scriptlet with the code that returns the 
registered Screens. These can be found in **script-tests** folder.

**In general the operations are as follows:**

Load and register the scriptlet with KWin scripting infrastructure, and gather the script registration number:
```bash
scriptNo=$(dbus-send --print-reply --dest=org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript string:nameOfTheFileContainingTheScriptlet | tail -1 | cut -c 10-11)
```

Mark the time at the script start, we will need this later when we query the journal for the script output:
```bash
et=$(date "+%Y-%m-%d %H:%M:%S.%N")
```

Run the script:
```bash
dbus-send --print-reply --dest=org.kde.KWin /Scripting/Script${scriptNo} org.kde.kwin.Script.run
```

Stop and deregister the script

```bash
dbus-send --print-reply --dest=org.kde.KWin /Scripting/Script${scriptNo} org.kde.kwin.Script.stop
```

Mark the time at script end, we will need this later when we query the journal for the script output:

```bash
et=$(date "+%Y-%m-%d %H:%M:%S.%N")
```

Gather the KWing script execution output in journal, for the time script was running
```bash
journalctl QT_CATEGORY=js QT_CATEGORY=kwin_scripting -o cat --since "${st}" --until "${et}" --no-pager
```

This is my first attempt in both JavaScript and KWin interfacing, so the above may not be the most optimal solution.
It may also stop working if KWin people change the KWin internals in future versions. For example, this 
will **NOT** work on **KWin5** and below, because the internal object/method names are different.

TODO: Add capabilities for setting window geometry/state. I don't seem to need that at this point though.

[^x11]: should also work in X11
