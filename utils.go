package main

import (
	"fmt"
	"strings"
)

func getDigitFromFontString(fontStr string) int {
	fontStr = strings.ToUpper(fontStr)[3:7]
	switch fontStr {
	case "E602", "E60E", "E618":
		return 1
	case "E605", "E610", "E617":
		return 2
	case "E604", "E611", "E61A":
		return 3
	case "E606", "E60C", "E619":
		return 4
	case "E607", "E60F", "E61B":
		return 5
	case "E608", "E612", "E61F":
		return 6
	case "E60A", "E613", "E61C":
		return 7
	case "E60B", "E614", "E61D":
		return 8
	case "E609", "E615", "E61E":
		return 9
	case "E603", "E60D", "E616":
		return 0
	default:
		fmt.Printf("not a valid font string: %s", fontStr)
		return 0
	}
}
