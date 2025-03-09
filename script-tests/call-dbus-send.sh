#!/bin/bash
clear

scriptFqfn="$(dirname "$(readlink -f "$0")")/scriptlet.js"
scriptNo=$(dbus-send --print-reply --dest=org.kde.KWin /Scripting org.kde.kwin.Scripting.loadScript string:"${scriptFqfn}" | tail -1 | cut -c 10-11)

st=$(date "+%Y-%m-%d %H:%M:%S.%N")
dbus-send --print-reply --dest=org.kde.KWin /Scripting/Script${scriptNo} org.kde.kwin.Script.run

dbus-send --print-reply --dest=org.kde.KWin /Scripting/Script${scriptNo} org.kde.kwin.Script.stop
et=$(date "+%Y-%m-%d %H:%M:%S.%N")

journalctl QT_CATEGORY=js QT_CATEGORY=kwin_scripting -o cat --since "${st}" --until "${et}" --no-pager

