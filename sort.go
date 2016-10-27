package main

type lengthwise []string

func (s lengthwise) Len() int {
	return len(s)
}
func (s lengthwise) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s lengthwise) Less(i, j int) bool {
	var iCount, jCount int
	for _, c := range s[i] {
		if c == '/' {
			iCount++
		}
	}
	for _, c := range s[j] {
		if c == '/' {
			jCount++
		}
	}
	if iCount != jCount {
		return iCount > jCount
	}
	return len(s[i]) > len(s[j])
}
