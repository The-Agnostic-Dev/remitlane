package seed

// Recipient names and payout details per country. Account numbers and
// mobile numbers stay clear of the sandbox's magic endings (docs/sandbox.md).
var names = map[string][]string{
	"NG": {"Chinedu Okafor", "Aisha Bello", "Tunde Adebayo", "Ngozi Eze", "Emeka Nwankwo", "Funmilayo Ojo",
		"Ibrahim Musa", "Kemi Adeyemi", "Obinna Uche", "Halima Sani", "Segun Bakare", "Chiamaka Obi"},
	"KE": {"Achieng Otieno", "Kamau Njoroge", "Mercy Wambui", "Otieno Odhiambo", "Faith Chebet", "Brian Kiprop",
		"Njeri Mwangi"},
	"GH": {"Kwame Boateng", "Akosua Mensah", "Yaw Owusu", "Abena Asante", "Kofi Appiah", "Esi Quaye",
		"Kwabena Darko"},
	"UG": {"Nakato Sarah", "Okello David", "Namubiru Grace", "Mugisha Ivan", "Atim Brenda", "Ssempala Joseph"},
}

var bankCodes = map[string][]string{
	"NG": {"058", "044", "033", "057", "011", "232"},
	"KE": {"01", "11", "68", "63", "03"},
	"GH": {"130100", "300304", "280100", "060100"},
	"UG": {"SBICUGKX", "DFCUUGKA", "CERBUGKA"},
}

var mobilePrefixes = map[string][]string{
	"KE": {"25471", "25472", "25479", "25411"},
	"GH": {"23324", "23354", "23355", "23327"},
	"UG": {"25677", "25678", "25670", "25675"},
}

var narrations = map[string][]string{
	"prt_kobopay":      {"Salary advance", "Wallet withdrawal", "Merchant settlement", "Refund"},
	"prt_dukamarket":   {"Seller payout", "Weekly seller settlement", "Rider earnings", "Refund to customer"},
	"prt_accrafreight": {"Driver payment", "Agent commission", "Fuel advance", "Port clearing fees"},
}
