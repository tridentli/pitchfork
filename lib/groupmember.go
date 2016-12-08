package pitchfork

type PfGroupMember interface {
	Set(groupname, username, fullname, affiliation string, groupadmin bool, groupstate, email, pgpkey_id, activity, tel, sms, airport string)
	GetGroupName() string
	GetUserName() string
	GetFullName() string
	GetEmail() string
	GetAffiliation() string
	GetGroupAdmin() bool
	GetGroupState() string
	GetPGPKeyID() string
	HasPGP() bool
	GetActivity() string
	GetTel() string
	GetSMS() string
	GetAirport() string
}

type PfGroupMemberS struct {
	UserName    string
	FullName    string
	Email       string
	Affiliation string
	GroupAdmin  bool
	GroupState  string
	GroupName   string
	PGPKeyID    string
	Activity    string
	Tel         string
	SMS         string
	Airport     string
}

func NewPfGroupMember() PfGroupMember {
	return &PfGroupMemberS{}
}

func (grpm *PfGroupMemberS) Set(groupname, username, fullname, affiliation string, groupadmin bool, groupstate, email, pgpkey_id, activity, telephone, sms, airport string) {
	grpm.GroupName = groupname
	grpm.UserName = username
	grpm.FullName = fullname
	grpm.Affiliation = affiliation
	grpm.GroupAdmin = groupadmin
	grpm.GroupState = groupstate
	grpm.Email = email
	grpm.PGPKeyID = pgpkey_id
	grpm.Activity = activity
	grpm.Tel = telephone
	grpm.SMS = sms
	grpm.Airport = airport
}

func (grpm *PfGroupMemberS) GetGroupName() string {
	return grpm.GroupName
}

func (grpm *PfGroupMemberS) GetUserName() string {
	return grpm.UserName
}

func (grpm *PfGroupMemberS) GetFullName() string {
	return grpm.FullName
}

func (grpm *PfGroupMemberS) GetEmail() string {
	return grpm.Email
}

func (grpm *PfGroupMemberS) GetAffiliation() string {
	return grpm.Affiliation
}

func (grpm *PfGroupMemberS) GetGroupAdmin() bool {
	return grpm.GroupAdmin
}

func (grpm *PfGroupMemberS) GetGroupState() string {
	return grpm.GroupState
}

func (grpm *PfGroupMemberS) GetPGPKeyID() string {
	return grpm.PGPKeyID
}

func (grpm *PfGroupMemberS) HasPGP() bool {
	return grpm.PGPKeyID != ""
}

func (grpm *PfGroupMemberS) GetActivity() string {
	return grpm.Activity
}

func (grpm *PfGroupMemberS) GetTel() string {
	return grpm.Tel
}

func (grpm *PfGroupMemberS) GetSMS() string {
	return grpm.SMS
}

func (grpm *PfGroupMemberS) GetAirport() string {
	return grpm.Airport
}
