package reptile

type token struct {
	ExpiresIn int `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	AccessToken string `json:"access_token"`
}

type companyInfoResult struct {
	Total int
	Count int
	ResultMsg string
	ResultCode int
	Records []companyInfo
}

type companyInfo struct {
	SECCODE string //证券代码
	SECNAME string //证券简称
}

type surveyResult struct {
	Success	bool
	Pages	int
	Data	[]survey
}

type survey struct {
	SCode		string
	//SName		string
	StartDate	string
	Description	string
}