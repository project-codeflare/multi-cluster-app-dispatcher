package genericresource

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func UnmarshalToUnstructured(raw []byte) (unstruct unstructured.Unstructured, err error) {
    unstruct.Object = make(map[string]interface{})
    var blob interface{}
    err = json.Unmarshal(raw, &blob)
    if err != nil {
        return unstruct, err
    }
    unstruct.Object = blob.(map[string]interface{})
    return unstruct, nil
}

func truncateName(name string) string {
	newName := name
	if len(newName) > 63 {
		newName = newName[:63]
	}
	return newName
}

func join(strs ...string) string {
	var result string
	if strs[0] == "" {
		return strs[len(strs)-1]
	}
	for _, str := range strs {
		result += str
	}
	return result
}

