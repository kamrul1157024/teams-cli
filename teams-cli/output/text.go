package output

import "fmt"

func Text(lines []string) {
	for _, line := range lines {
		fmt.Println(line)
	}
}

func Line(s string) {
	fmt.Println(s)
}
