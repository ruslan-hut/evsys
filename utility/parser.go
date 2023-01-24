package utility

import (
	"encoding/json"
)

func ParseJson(b []byte) ([]interface{}, error) {
	var array []interface{}
	err := json.Unmarshal(b, &array)
	return array, err
}
