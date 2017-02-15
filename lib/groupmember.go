package pitchfork

// PfGroupMember provides an interface for modifying a Group Member
type PfGroupMember interface {
	SQL_Selects() string
	SQL_Froms() string
	SQL_Scan(rows *Rows) (err error)
	Set(groupname, groupdesc, username, fullname, affiliation string, groupadmin bool, groupstate string, cansee bool, email, pgpkey_id, entered, activity, telephone, sms, airport string)
	GetGroupName() string
	GetGroupDesc() string
	GetUserName() string
	GetFullName() string
	GetEmail() string
	GetAffiliation() string
	GetGroupAdmin() bool
	GetGroupState() string
	GetGroupCanSee() bool
	GetPGPKeyID() string
	HasPGP() bool
	GetEntered() string
	GetActivity() string
	GetTel() string
	GetSMS() string
	GetAirport() string
}

// PfGroupMemberS is the implementation of a PfGroupMember
type PfGroupMemberS struct {
	UserName    string
	FullName    string
	Email       string
	Affiliation string
	GroupAdmin  bool
	GroupState  string
	GroupCanSee bool
	GroupName   string
	GroupDesc   string
	PGPKeyID    string
	Entered     string
	Activity    string
	Tel         string
	SMS         string
	Airport     string
}

// NewPfGroupMember creates a new PfGroupMember
func NewPfGroupMember() PfGroupMember {
	return &PfGroupMemberS{}
}

// SQL_Selects returns the SQL SELECT statement needed to query common properties
func (grpm *PfGroupMemberS) SQL_Selects() (q string) {
	return "SELECT " +
		"m.ident, " +
		"m.descr, " +
		"m.affiliation, " +
		"mt.trustgroup, " +
		"grp.descr, " +
		"mt.admin, " +
		"mt.state, " +
		"ms.can_see, " +
		"mt.email, " +
		"me.pgpkey_id, " +
		"DATE_TRUNC('days', AGE(mt.entered)), " +
		"EXTRACT(day FROM now() - m.activity) as activity, " +
		"m.tel_info, " +
		"m.sms_info, " +
		"m.airport"
}

// SQL_Froms returns the SQL FROM portion
func (grpm *PfGroupMemberS) SQL_Froms() string {
	return "FROM member_trustgroup mt " +
		"INNER JOIN trustgroup grp ON (mt.trustgroup = grp.ident) " +
		"INNER JOIN member m ON (mt.member = m.ident) " +
		"INNER JOIN member_state ms ON (ms.ident = mt.state) " +
		"INNER JOIN member_email me ON (me.email = mt.email)"
}

// SQL_Scan scans a Row for GroupMembers
func (grpm *PfGroupMemberS) SQL_Scan(rows *Rows) (err error) {
	return rows.Scan(
		&grpm.UserName,
		&grpm.FullName,
		&grpm.Affiliation,
		&grpm.GroupName,
		&grpm.GroupDesc,
		&grpm.GroupAdmin,
		&grpm.GroupState,
		&grpm.GroupCanSee,
		&grpm.Email,
		&grpm.PGPKeyID,
		&grpm.Entered,
		&grpm.Activity,
		&grpm.Tel,
		&grpm.SMS,
		&grpm.Airport)
}

// Set configures properties of a groupmember
func (grpm *PfGroupMemberS) Set(groupname, groupdesc, username, fullname, affiliation string, groupadmin bool, groupstate string, cansee bool, email, pgpkey_id, entered, activity, telephone, sms, airport string) {
	grpm.GroupName = groupname
	grpm.GroupDesc = groupdesc
	grpm.UserName = username
	grpm.FullName = fullname
	grpm.Affiliation = affiliation
	grpm.GroupAdmin = groupadmin
	grpm.GroupState = groupstate
	grpm.GroupCanSee = cansee
	grpm.Email = email
	grpm.PGPKeyID = pgpkey_id
	grpm.Entered = entered
	grpm.Activity = activity
	grpm.Tel = telephone
	grpm.SMS = sms
	grpm.Airport = airport
}

// GetGroupName returns the name of the group.
func (grpm *PfGroupMemberS) GetGroupName() string {
	return grpm.GroupName
}

// returns the group description.
func (grpm *PfGroupMemberS) GetGroupDesc() string {
	return grpm.GroupDesc
}

// GetUserName returns the username of the groupmember.
func (grpm *PfGroupMemberS) GetUserName() string {
	return grpm.UserName
}

// GetFullName gets the full name of the groupmember.
func (grpm *PfGroupMemberS) GetFullName() string {
	return grpm.FullName
}

// Returns the email address of a groupmember.
func (grpm *PfGroupMemberS) GetEmail() string {
	return grpm.Email
}

// GetAffiliation gets the affiliation of a group member
func (grpm *PfGroupMemberS) GetAffiliation() string {
	return grpm.Affiliation
}

// GetGroupAdmin sets the group admin bit
func (grpm *PfGroupMemberS) GetGroupAdmin() bool {
	return grpm.GroupAdmin
}

// GetGroupState gets the state of a group
func (grpm *PfGroupMemberS) GetGroupState() string {
	return grpm.GroupState
}

// Returns the CanSee attribute of a groupmember.
func (grpm *PfGroupMemberS) GetGroupCanSee() bool {
	return grpm.GroupCanSee
}

// Returns the PGPKeyID of a groupmember.
func (grpm *PfGroupMemberS) GetPGPKeyID() string {
	return grpm.PGPKeyID
}

// Returns whether the user has a PGP key.
func (grpm *PfGroupMemberS) HasPGP() bool {
	return grpm.PGPKeyID != ""
}

// Returns the 'entered' attribute of a groupmember.
func (grpm *PfGroupMemberS) GetEntered() string {
	return grpm.Entered
}

// Returns the last activity of a groupmember.
func (grpm *PfGroupMemberS) GetActivity() string {
	return grpm.Activity
}

// Returns the Telephone number of a groupmember.
func (grpm *PfGroupMemberS) GetTel() string {
	return grpm.Tel
}

// Returns the SMS number of a groupmember.
func (grpm *PfGroupMemberS) GetSMS() string {
	return grpm.SMS
}

// Returns the Airport code of a groupmember.
func (grpm *PfGroupMemberS) GetAirport() string {
	return grpm.Airport
}
