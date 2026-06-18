package conv

func ToString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
