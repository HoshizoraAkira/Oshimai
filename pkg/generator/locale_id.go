package generator

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
)

// indonesianNames is a small pool of realistic Nusantara names used in place of generic
// "Standard Item" / "shopper_01" placeholders when Locale is "id".
var indonesianNames = []string{
	"Budi Santoso", "Siti Nurhaliza", "Agus Setiawan", "Dewi Lestari",
	"Muhammad Iqbal", "Putri Ayu Wulandari", "Rizky Ramadhan", "Ayu Kusuma",
}

var indonesianCities = []string{
	"Jakarta Selatan", "Surabaya", "Bandung", "Medan", "Semarang", "Makassar", "Denpasar", "Yogyakarta",
}

var indonesianStreets = []string{
	"Jl. Sudirman No. 45", "Jl. Gatot Subroto Kav. 12", "Jl. Diponegoro No. 8", "Jl. Ahmad Yani No. 101",
}

// LocalizeIndonesian walks a JSON payload (as produced by SynthesizePayload) and replaces
// generically-synthesized field values with Indonesian-locale equivalents (nama Nusantara, nomor
// HP +62, NIK 16-digit, alamat kota Indonesia) based on the same field-name heuristics used during
// synthesis. It is a best-effort post-processing pass: payloads that fail to parse as JSON, or
// that are not a JSON object/array, are returned unchanged.
func LocalizeIndonesian(payloadJSON string) string {
	if strings.TrimSpace(payloadJSON) == "" {
		return payloadJSON
	}

	var val any
	if err := json.Unmarshal([]byte(payloadJSON), &val); err != nil {
		return payloadJSON
	}

	localized := localizeValue(val)

	out, err := json.MarshalIndent(localized, "", "  ")
	if err != nil {
		return payloadJSON
	}
	return string(out)
}

func localizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		result := make(map[string]any, len(t))
		for k, fieldVal := range t {
			if nested, ok := fieldVal.(map[string]any); ok {
				result[k] = localizeValue(nested)
				continue
			}
			if arr, ok := fieldVal.([]any); ok {
				result[k] = localizeValue(arr)
				continue
			}
			if str, ok := fieldVal.(string); ok {
				result[k] = localizeStringField(k, str)
				continue
			}
			result[k] = fieldVal
		}
		return result
	case []any:
		result := make([]any, len(t))
		for i, item := range t {
			result[i] = localizeValue(item)
		}
		return result
	default:
		return v
	}
}

func localizeStringField(fieldName, original string) string {
	f := strings.ToLower(fieldName)

	switch {
	case strings.Contains(f, "email"):
		return "pelanggan@contoh.co.id"
	case strings.Contains(f, "phone") || strings.Contains(f, "hp") || strings.Contains(f, "telp") || strings.Contains(f, "mobile"):
		return fmt.Sprintf("+62812%08d", rand.Intn(100000000))
	case strings.Contains(f, "nik") || strings.Contains(f, "ktp"):
		return fmt.Sprintf("35%014d", rand.Intn(100000000000000))
	case strings.Contains(f, "city") || strings.Contains(f, "kota"):
		return indonesianCities[rand.Intn(len(indonesianCities))]
	case strings.Contains(f, "address") || strings.Contains(f, "alamat"):
		return indonesianStreets[rand.Intn(len(indonesianStreets))]
	case strings.Contains(f, "username") || strings.Contains(f, "user"):
		return "pengguna_01"
	case strings.Contains(f, "name") || strings.Contains(f, "nama"):
		return indonesianNames[rand.Intn(len(indonesianNames))]
	case strings.Contains(f, "password") || strings.Contains(f, "secret") || strings.Contains(f, "token") || strings.Contains(f, "id"):
		return original // Leave credential/id-shaped fields untouched — locale is irrelevant there.
	default:
		return original
	}
}
