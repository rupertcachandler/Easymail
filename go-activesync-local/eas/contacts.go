package eas

// Contact is the sync representation of a contact (MS-ASCNTC 14.1).
type Contact struct {
	XMLName             struct{} `wbxml:"AirSync.ApplicationData"`
	FirstName           string   `wbxml:"Contacts.FirstName,omitempty"`
	LastName            string   `wbxml:"Contacts.LastName,omitempty"`
	MiddleName          string   `wbxml:"Contacts.MiddleName,omitempty"`
	Title               string   `wbxml:"Contacts.Title,omitempty"`
	CompanyName         string   `wbxml:"Contacts.CompanyName,omitempty"`
	JobTitle            string   `wbxml:"Contacts.JobTitle,omitempty"`
	Email1Address       string   `wbxml:"Contacts.Email1Address,omitempty"`
	Email2Address       string   `wbxml:"Contacts.Email2Address,omitempty"`
	Email3Address       string   `wbxml:"Contacts.Email3Address,omitempty"`
	HomePhoneNumber     string   `wbxml:"Contacts.HomePhoneNumber,omitempty"`
	MobilePhoneNumber   string   `wbxml:"Contacts.MobilePhoneNumber,omitempty"`
	BusinessPhoneNumber string   `wbxml:"Contacts.BusinessPhoneNumber,omitempty"`
	HomeStreet          string   `wbxml:"Contacts.HomeStreet,omitempty"`
	HomeCity            string   `wbxml:"Contacts.HomeCity,omitempty"`
	HomePostalCode      string   `wbxml:"Contacts.HomePostalCode,omitempty"`
	HomeCountry         string   `wbxml:"Contacts.HomeCountry,omitempty"`
}
