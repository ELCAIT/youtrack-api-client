package youtrack

// MailServer represents YouTrack email settings.
type MailServer struct {
	IsEnabled    bool   `json:"isEnabled"`
	MailProtocol string `json:"mailProtocol"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Anonymous    bool   `json:"anonymous"`
	Login        string `json:"login"`
	From         string `json:"from"`
	ReplyTo      string `json:"replyTo"`
}

// MailServerResponse represents the API response structure.
type MailServerResponse struct {
	MailServer MailServer `json:"emailSettings"`
}

// SystemSettings represents youtrack system settings.
type SystemSettings struct {
	AdministratorEmail        string `json:"administratorEmail"`
	MaxExportItems            int    `json:"maxExportItems"`
	MaxUploadFileSize         int    `json:"maxUploadFileSize"`
	AllowStatisticsCollection bool   `json:"allowStatisticsCollection"`
	IsApplicationReadOnly     bool   `json:"isApplicationReadOnly"`
	BaseUrl                   string `json:"baseUrl"`
}

// LocaleDescriptor represents a locale configuration.
type LocaleDescriptor struct {
	ID        string `json:"id"`
	Locale    string `json:"locale"`
	Language  string `json:"language"`
	Community bool   `json:"community"`
	Name      string `json:"name"`
}

// LocaleSettings represents youtrack locale settings.
type LocaleSettings struct {
	Locale LocaleDescriptor `json:"locale"`
}

// DateFormatDescriptor represents the date format settings in YouTrack.
type DateFormatDescriptor struct {
	ID           string `json:"id"`
	Presentation string `json:"presentation"`
	Pattern      string `json:"pattern"`
	DatePattern  string `json:"datePattern"`
}

// TimeZoneDescriptor represents the time zone settings in YouTrack.
type TimeZoneDescriptor struct {
	ID           string `json:"id"`
	Presentation string `json:"presentation"`
	Offset       int    `json:"offset"`
}

// AppearanceSettings represents youtrack appearance settings.
type AppearanceSettings struct {
	ID         string               `json:"id"`
	DateFormat DateFormatDescriptor `json:"dateFieldFormat"`
	TimeZone   TimeZoneDescriptor   `json:"timeZone"`
}

type License struct {
	Type     string `json:"$type"`
	ID       string `json:"id,omitempty"`
	License  string `json:"license"`
	Username string `json:"username,omitempty"`
	Error    string `json:"error,omitempty"`
}

// GlobalSettings represents youtrack global settings.
type GlobalSettings struct {
	ID      string   `json:"id"`
	License *License `json:"license"`
}

// RestSettings represents YouTrack REST API settings.
type RestSettings struct {
	AllowAllOrigins bool     `json:"allowAllOrigins"`
	AllowedOrigins  []string `json:"allowedOrigins"`
	ID              string   `json:"id"`
}

// BackupSettings represents YouTrack backup settings.
type BackupSettings struct {
	ID                 string `json:"id"`
	Location           string `json:"location"`
	FilesToKeep        int    `json:"filesToKeep"`
	CronExpression     string `json:"cronExpression"`
	ArchiveFormat      string `json:"archiveFormat"`
	Enabled            bool   `json:"isOn"`
	AvailableDiskSpace int64  `json:"availableDiskSpace"`
	NotifiedUsers      []User `json:"notifiedUsers"`
}

// WorkItemType represents a global work item type in YouTrack time tracking settings.
type WorkItemType struct {
	ID           string `json:"id,omitempty"`
	Name         string `json:"name"`
	AutoAttached bool   `json:"autoAttached"`
}

// WorkTimeSettings represents system-wide work schedule settings.
type WorkTimeSettings struct {
	ID             string `json:"id,omitempty"`
	MinutesADay    int    `json:"minutesADay"`
	WorkDays       []int  `json:"workDays"`
	FirstDayOfWeek int    `json:"firstDayOfWeek,omitempty"`
	DaysAWeek      int    `json:"daysAWeek,omitempty"`
}

// WorkItemAttributeValue represents a value in a work item attribute prototype.
type WorkItemAttributeValue struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AutoAttach  bool   `json:"autoAttach"`
}

// WorkItemProjectAttribute represents project-specific settings of an attribute prototype.
type WorkItemProjectAttribute struct {
	ID      string                   `json:"id,omitempty"`
	Name    string                   `json:"name,omitempty"`
	Ordinal int                      `json:"ordinal,omitempty"`
	Values  []WorkItemAttributeValue `json:"values,omitempty"`
}

// WorkItemAttributePrototype represents a global work item attribute prototype.
type WorkItemAttributePrototype struct {
	ID        string                     `json:"id,omitempty"`
	Name      string                     `json:"name"`
	Values    []WorkItemAttributeValue   `json:"values,omitempty"`
	Instances []WorkItemProjectAttribute `json:"instances,omitempty"`
}

// GlobalTimeTrackingSettings represents the root global time tracking settings entity.
type GlobalTimeTrackingSettings struct {
	ID                  string                       `json:"id,omitempty"`
	WorkTimeSettings    WorkTimeSettings             `json:"workTimeSettings"`
	WorkItemTypes       []WorkItemType               `json:"workItemTypes,omitempty"`
	AttributePrototypes []WorkItemAttributePrototype `json:"attributePrototypes,omitempty"`
}
