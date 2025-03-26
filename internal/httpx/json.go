// Package httpx holds small HTTP helpers shared by the API, the console and
// the provider clients.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// MaxBody is the largest request body we read.
const MaxBody = 1 << 20

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// DecodeJSON reads a JSON body into v.
func DecodeJSON(r *http.Request, v interface{}) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBody+1))
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	if len(body) > MaxBody {
		return errors.New("request body is too large")
	}
	if err := json.Unmarshal(body, v); err != nil {
		var syn *json.SyntaxError
		if errors.As(err, &syn) {
			return fmt.Errorf("request body is not valid JSON (at byte %d)", syn.Offset)
		}
		var typ *json.UnmarshalTypeError
		if errors.As(err, &typ) {
			return fmt.Errorf("field %q must be a %s", typ.Field, typ.Type)
		}
		return err
	}
	return nil
}
