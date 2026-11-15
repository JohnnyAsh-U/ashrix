package config

type Config struct {
    Connector    Connector     `yaml:"connector" mapstructure:"connector"`
    Gateway      Gateway       `yaml:"gateway" mapstructure:"gateway"`
    Applications []Application `yaml:"applications" mapstructure:"applications"`
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
    ServerName string `yaml:"server_name" mapstructure:"server_name"`
}

type Application struct {
    Name   string `yaml:"name" mapstructure:"name"`
    Target string `yaml:"target" mapstructure:"target"`
}


type Logging struct {
    Level  string `yaml:"level" mapstructure:"level"`
    Format string `yaml:"format" mapstructure:"format"`
	Dir string `yaml:"dir"`
}
