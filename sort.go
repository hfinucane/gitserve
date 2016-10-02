package main

type Lengthwise []string

func (s Lengthwise) Len() int {
	return len(s)
}
func (s Lengthwise) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s Lengthwise) Less(i, j int) bool {
	var i_slash_count, j_slash_count int
	for _, c := range s[i] {
		if c == '/' {
			i_slash_count++
		}
	}
	for _, c := range s[j] {
		if c == '/' {
			j_slash_count++
		}
	}
	if i_slash_count != j_slash_count {
		return i_slash_count > j_slash_count
	}
	return len(s[i]) > len(s[j])
}
