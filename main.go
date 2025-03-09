package main

import (
	"fmt"
	"github.com/google/uuid"
	"log"
	"sort"
	"strings"

	"github.com/samber/lo"
)

// Example of KWin usage
func printEnvironment(env Environment) {
	fmt.Printf("Screens %d, left to right:\n", len(env.Screens))
	scr := lo.MapToSlice(env.Screens, func(key string, value Screen) Screen {
		return value
	})
	sort.Slice(scr, func(i, j int) bool {
		return scr[i].Geometry.TopLeft.X < scr[j].Geometry.TopLeft.X
	})
	for _, s := range scr {
		fmt.Printf("\tName: %s; ", s.Name)
		fmt.Printf("Model: %s; ", s.Model)
		fmt.Printf("Manufacturer: %s; ", s.Manufacturer)
		fmt.Printf("Serial: %s; ", s.SerialNumber)
		fmt.Printf("Pixel ratio: %.2f; ", s.PixelRatio)
		fmt.Printf("Geometry: %+v\n", s.Geometry)
	}
	fmt.Printf("Desktops: %d\n", len(env.Desktops))
	ds := lo.MapToSlice(env.Desktops, func(key uuid.UUID, value Desktop) Desktop {
		return value
	})
	sort.Slice(ds, func(i, j int) bool {
		return ds[i].X11Number < ds[j].X11Number
	})
	for _, d := range ds {
		fmt.Printf("\tX11 Number: %d; ", d.X11Number)
		fmt.Printf("Name: %s; ", d.Name)
		fmt.Printf("ID: %s\n", d.Id)
	}
	fmt.Printf("Windows: %d\n", len(env.Windows))
	for _, w := range env.Windows {
		fmt.Printf("Caption: %s\n", w.Caption)
		fmt.Printf("\tID: %s\n", w.Id)
		fmt.Printf("\tPID: %d\n", w.Pid)
		fmt.Printf("\tCmd: %s\n", w.CmdLine)
		fmt.Printf("\tApp: %s\n", w.AppName)
		fmt.Printf("\tGeom: [X:%.2f,Y:%.2f,W:%.2f,H:%.2f]; ", w.X, w.Y, w.Width, w.Height)
		fmt.Printf("Fullscreen: %t; ", w.Fullscreen)
		fmt.Printf("OnAllDesktops: %t; ", w.OnAllDesktops)
		fmt.Printf("KeepAbove: %t; ", w.KeepAbove)
		fmt.Printf("KeepBelow: %t; ", w.KeepBelow)
		fmt.Printf("Minimized: %t\n", w.Minimized)
		desktopNames := make([]string, len(w.Desktops))
		for i, d := range w.Desktops {
			desktopNames[i] = d.Name
		}
		fmt.Printf("\tOn desktops: %s\n", strings.Join(desktopNames, ", "))
	}
}

func main() {
	kw := NewKWin()
	env, err := kw.GetEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	printEnvironment(env)
}
