package user

type OpenPGPKeys struct {
	PrivateKey            string `json:"privateKey"`
	PublicKey             string `json:"publicKey"`
	RevocationCertificate string `json:"revocationCertificate"`
}
type SystemEmail struct {
	IsSystemEmail    bool   `json:"is_system_email"`
	SystemEmailReply string `json:"system_email_reply"`
}
type UserConfig struct {
	ID          string      `json:"id"`
	Email       string      `json:"email"`
	Hash        string      `json:"hash"`
	TenantName  string      `json:"tenant_name"`
	MailboxSize int         `json:"mailbox_size"`
	Usage       int         `json:"usage"`
	Key         string      `json:"key"`
	OpenPGP     OpenPGPKeys `json:"open_pgp"`
	SystemEmail SystemEmail `json:"system_email"`
}
