//go:build hosted

package config

type hosted struct {
	UnkeyRootKey string
	MPPSecretKey string
	MPPRecipient string
	MPPRPCURL    string
	MPPRealm     string
	MPPAmount    string
}

func loadHosted(c *Config) {
	c.UnkeyRootKey = env("UNKEY_ROOT_KEY", "")
	c.MPPSecretKey = env("MPP_SECRET_KEY", "")
	c.MPPRecipient = env("MPP_RECIPIENT", "")
	c.MPPRPCURL = env("MPP_RPC_URL", "https://rpc.moderato.tempo.xyz")
	c.MPPRealm = env("MPP_REALM", "klimatsearch")
	c.MPPAmount = env("MPP_AMOUNT", "0.01")
}

// Hosted is true when the binary was built with -tags hosted.
func (Config) Hosted() bool { return true }
