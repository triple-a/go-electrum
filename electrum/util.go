package electrum

import (
	"encoding/json"
	"fmt"
)

func mustMarshalJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%+v", v)
	}
	return string(b[:])
}
