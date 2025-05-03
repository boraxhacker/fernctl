package ssm

import (
	"fmt"
)

func flatten(data map[string]interface{}, prefix string) map[string]interface{} {

	var result = make(map[string]interface{})

	for key, value := range data {
		newKey := prefix + key
		switch value.(type) {
		case map[string]interface{}:
			jvals := flatten(value.(map[string]interface{}), newKey+"/")
			for jkey, jval := range jvals {
				result[jkey] = jval
			}
		case []interface{}:
			for i, v := range value.([]interface{}) {
				newKey := fmt.Sprintf("%s/%d/", newKey, i)
				jvals := flatten(v.(map[string]interface{}), newKey)
				for jkey, jval := range jvals {
					result[jkey] = jval
				}
			}
		default:
			result[newKey] = value.(string)
		}
	}

	return result
}
