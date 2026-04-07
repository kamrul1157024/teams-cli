package output

import (
	"encoding/json"
	"fmt"
	"os"
)

func JSON(data any, pretty bool) error {
	encoder := json.NewEncoder(os.Stdout)
	if pretty {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(data)
}

func JSONString(data any, pretty bool) (string, error) {
	var b []byte
	var err error
	if pretty {
		b, err = json.MarshalIndent(data, "", "  ")
	} else {
		b, err = json.Marshal(data)
	}
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func Errorf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}
