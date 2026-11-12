package config

type Config struct {
    Connector    Connector     `yaml:"connector" mapstructure:"connector"`
    Gateway      Gateway       `yaml:"gateway" mapstructure:"gateway"`
    Applications []Application `yaml:"applications" mapstructure:"applications"`
    Metrics      Metrics       `yaml:"metrics" mapstructure:"metrics"`
    Logging      Logging       `yaml:"logging" mapstructure:"logging"`
}

type Connector struct {
    ID          string `yaml:"id" mapstructure:"id"`
    Certificate string `yaml:"certificate" mapstructure:"certificate"`
    PrivateKey  string `yaml:"private_key" mapstructure:"private_key"`
    CA          string `yaml:"ca" mapstructure:"ca"`
}


type Gateway struct {
    Address string `yaml:"address" mapstructure:"address"`
}

type Application struct {
    Name   string `yaml:"name" mapstructure:"name"`
    Target string `yaml:"target" mapstructure:"target"`
}

type Metrics struct {
    Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
    Listen  string `yaml:"listen" mapstructure:"listen"`
}

type Logging struct {
    Level  string `yaml:"level" mapstructure:"level"`
    Format string `yaml:"format" mapstructure:"format"`
	Dir string `yaml:"dir"`
}
