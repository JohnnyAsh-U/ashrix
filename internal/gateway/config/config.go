package config


type Config struct {
	Gateway     Gateway       `yaml:"gateway"`
	Listeners   Listeners     `yaml:"listeners"`
	PKI         PKI           `yaml:"pki"`
	Connectors  []Connector   `yaml:"connectors"`
	Applications []Application `yaml:"applications"`
	Admin       Admin         `yaml:"admin"`
	Logging     Logging       `yaml:"logging"`
}

type Gateway struct {
	ID string `yaml:"id"`
	Certificate string `yaml:"certificate"`
	PrivateKey string `yaml:"private_key" mapstructure:"private_key"`
}

type Listeners struct {
	HTTP  HTTPListener  `yaml:"http"`
	HTTPS HTTPSListener `yaml:"https"`
	QUIC  QUICListener  `yaml:"quic"`
}

type HTTPListener struct {
	Address string `yaml:"address"`
}

type HTTPSListener struct {
	Address     string `yaml:"address"`
	Certificate string `yaml:"certificate"`
	PrivateKey  string `yaml:"private_key" mapstructure:"private_key"`
}

type QUICListener struct {
	Address string `yaml:"address"`
}

type PKI struct {
	CACertificate       string `yaml:"ca_certificate" mapstructure:"ca_certificate"`
	CAPrivateKey        string `yaml:"ca_private_key" mapstructure:"ca_private_key"`
}

type Connector struct {
	ID          string      `yaml:"id"`
	CertificateSerial string `yaml:"certificate_serial" mapstructure:"certificate_serial"`
}


type Application struct {
	Name       string `yaml:"name"`
	Domain     string `yaml:"domain"`
	Connector  string `yaml:"connector"`
}

type Admin struct {
	Address string `yaml:"address"`
}

type Logging struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Dir string `yaml:"dir"`
}
