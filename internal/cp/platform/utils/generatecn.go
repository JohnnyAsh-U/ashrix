package utils

func GenerateCommonName(node_type string) string {
	switch node_type {
	case "gateway":
		return "gw" + GenerateRandomString(7)
	case "connector":
		return "cn" + GenerateRandomString(6)
	default:
		return ""
	}
}
