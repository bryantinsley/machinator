package config

// StripJSONComments removes // comments from JSONC
func StripJSONComments(data []byte) []byte {
	var result []byte
	inString := false
	i := 0
	for i < len(data) {
		if data[i] == '"' && (i == 0 || data[i-1] != '\\') {
			inString = !inString
		}
		if !inString && i+1 < len(data) && data[i] == '/' && data[i+1] == '/' {
			for i < len(data) && data[i] != '\n' {
				i++
			}
			continue
		}
		result = append(result, data[i])
		i++
	}
	return result
}
