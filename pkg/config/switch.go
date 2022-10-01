package config

import (
	"flag"
	"github.com/luscis/openlan/pkg/libol"
	"path/filepath"
)

func DefaultPerf() *Perf {
	return &Perf{
		Point:    64,
		Neighbor: 64,
		OnLine:   64,
		Link:     64,
		User:     1024,
		Esp:      64,
		State:    64 * 4,
		Policy:   64 * 8,
		VxLAN:    64,
	}
}

type Perf struct {
	Point    int `json:"point"`
	Neighbor int `json:"neighbor"`
	OnLine   int `json:"online"`
	Link     int `json:"link"`
	User     int `json:"user"`
	Esp      int `json:"esp"`
	State    int `json:"state"`
	Policy   int `json:"policy"`
	VxLAN    int `json:"vxlan"`
}

func (p *Perf) Correct(obj *Perf) {
	if p.Point == 0 && obj != nil {
		p.Point = obj.Point
	}
	if p.Neighbor == 0 && obj != nil {
		p.Neighbor = obj.Neighbor
	}
	if p.OnLine == 0 && obj != nil {
		p.OnLine = obj.OnLine
	}
	if p.Link == 0 && obj != nil {
		p.Link = obj.Link
	}
	if p.User == 0 && obj != nil {
		p.User = obj.User
	}
	if p.Esp == 0 && obj != nil {
		p.Esp = obj.Esp
	}
	if p.State == 0 && obj != nil {
		p.State = obj.State
	}
	if p.Policy == 0 && obj != nil {
		p.Policy = obj.Policy
	}
	if p.VxLAN == 0 && obj != nil {
		p.VxLAN = obj.VxLAN
	}
}

type Switch struct {
	File      string     `json:"file"`
	Alias     string     `json:"alias"`
	Perf      Perf       `json:"limit,omitempty"`
	Protocol  string     `json:"protocol"` // tcp, tls, udp, kcp, ws and wss.
	Listen    string     `json:"listen"`
	Timeout   int        `json:"timeout"`
	Http      *Http      `json:"http,omitempty"`
	Log       Log        `json:"log"`
	Cert      *Cert      `json:"cert,omitempty"`
	Crypt     *Crypt     `json:"crypt,omitempty"`
	Network   []*Network `json:"network,omitempty"`
	Acl       []*ACL     `json:"acl,omitempty"`
	FireWall  []FlowRule `json:"firewall,omitempty"`
	Inspect   []string   `json:"inspect,omitempty"`
	Queue     Queue      `json:"queue"`
	PassFile  string     `json:"password"`
	Ldap      *LDAP      `json:"ldap,omitempty"`
	AddrPool  string     `json:"pool,omitempty"`
	ConfDir   string     `json:"-"`
	TokenFile string     `json:"-"`
}

func DefaultSwitch() *Switch {
	obj := &Switch{
		Timeout: 120,
		Log: Log{
			File:    LogFile("openlan-switch.log"),
			Verbose: libol.INFO,
		},
		Http: &Http{
			Listen: "0.0.0.0:10000",
		},
		Listen: "0.0.0.0:10002",
		Cert:   &Cert{},
		Crypt:  &Crypt{},
	}
	obj.Correct(nil)
	return obj
}

func NewSwitch() *Switch {
	s := Manager.Switch
	s.Flags()
	s.Parse()
	s.Initialize()
	return s
}

func (s *Switch) Flags() {
	obj := DefaultSwitch()
	flag.StringVar(&s.Log.File, "log:file", obj.Log.File, "Configure log file")
	flag.StringVar(&s.ConfDir, "conf:dir", obj.ConfDir, "Configure switch's directory")
	flag.IntVar(&s.Log.Verbose, "log:level", obj.Log.Verbose, "Configure log level")
}

func (s *Switch) Parse() {
	flag.Parse()
}

func (s *Switch) Initialize() {
	s.File = s.Dir("switch.json")
	if err := s.Load(); err != nil {
		libol.Error("Switch.Initialize %s", err)
	}
	s.Default()
	libol.Debug("Switch.Initialize %v", s)
}

func (s *Switch) Correct(obj *Switch) {
	if s.Alias == "" {
		s.Alias = GetAlias()
	}
	CorrectAddr(&s.Listen, 10002)
	if s.Http != nil {
		CorrectAddr(&s.Http.Listen, 10000)
	}
	libol.Debug("Proxy.Correct Http %v", s.Http)
	s.TokenFile = filepath.Join(s.ConfDir, "token")
	s.File = filepath.Join(s.ConfDir, "switch.json")
	if s.Cert == nil {
		s.Cert = obj.Cert
	} else {
		s.Cert.Correct()
	}
	if s.Crypt == nil {
		s.Crypt = obj.Crypt
	}
	perf := &s.Perf
	perf.Correct(DefaultPerf())
	s.PassFile = filepath.Join(s.ConfDir, "password")
	if s.Protocol == "" {
		s.Protocol = "tcp"
	}
	if s.AddrPool == "" {
		s.AddrPool = "100.44"
	}
}

func (s *Switch) Dir(elem ...string) string {
	args := append([]string{s.ConfDir}, elem...)
	return filepath.Join(args...)
}

func (s *Switch) Format() {
	for _, obj := range s.Network {
		libol.Debug("Switch.Format %s", obj)
		context := obj.Specifies
		switch obj.Provider {
		case "esp":
			obj.Specifies = &ESPSpecifies{}
		case "vxlan":
			obj.Specifies = &VxLANSpecifies{}
		case "fabric":
			obj.Specifies = &FabricSpecifies{}
		default:
			obj.Specifies = nil
			continue
		}
		if data, err := libol.Marshal(context, true); err == nil {
			if err := libol.Unmarshal(obj.Specifies, data); err != nil {
				libol.Warn("Switch.Format %s", err)
			}
		}
	}
}

func (s *Switch) LoadNetwork() {
	files, err := filepath.Glob(s.Dir("network", "*.json"))
	if err != nil {
		libol.Error("Switch.LoadNetwork %s", err)
	}
	for _, k := range files {
		obj := &Network{
			Alias:   s.Alias,
			File:    k,
			ConfDir: s.ConfDir,
		}
		if err := libol.UnmarshalLoad(obj, k); err != nil {
			libol.Error("Switch.LoadNetwork %s", err)
			continue
		}
		obj.LoadLink()
		obj.LoadRoute()
		s.Network = append(s.Network, obj)
	}
	s.Format()
	for _, obj := range s.Network {
		for _, link := range obj.Links {
			link.Default()
		}
		obj.Correct()
		obj.Alias = s.Alias
		if obj.File == "" {
			obj.File = s.Dir("network", obj.Name+".json")
		}
	}
}

func (s *Switch) LoadAcl() {
	files, err := filepath.Glob(s.Dir("acl", "*.json"))
	if err != nil {
		libol.Error("Switch.LoadAcl %s", err)
	}
	for _, k := range files {
		obj := &ACL{
			File: k,
		}
		if err := libol.UnmarshalLoad(obj, k); err != nil {
			libol.Error("Switch.LoadAcl %s", err)
			continue
		}
		s.Acl = append(s.Acl, obj)
	}
	for _, obj := range s.Acl {
		for _, rule := range obj.Rules {
			rule.Correct()
		}
		if obj.File == "" {
			obj.File = s.Dir("acl", obj.Name+".json")
		}
	}
}

func (s *Switch) Default() {
	obj := DefaultSwitch()
	s.Correct(obj)
	if s.Timeout == 0 {
		s.Timeout = obj.Timeout
	}
	if s.Crypt != nil {
		s.Crypt.Default()
	}
	queue := &s.Queue
	queue.Default()
	s.LoadAcl()
	s.LoadNetwork()
}

func (s *Switch) Load() error {
	return libol.UnmarshalLoad(s, s.File)
}

func (s *Switch) Save() {
	tmp := *s
	tmp.Acl = nil
	tmp.Network = nil
	if err := libol.MarshalSave(&tmp, tmp.File, true); err != nil {
		libol.Error("Switch.Save %s", err)
	}
	s.SaveAcl()
	s.SaveNets()
}

func (s *Switch) SaveAcl() {
	if s.Acl == nil {
		return
	}
	for _, obj := range s.Acl {
		if err := libol.MarshalSave(obj, obj.File, true); err != nil {
			libol.Error("Switch.Save.Acl %s %s", obj.Name, err)
		}
	}
}

func (s *Switch) SaveNets() {
	if s.Network == nil {
		return
	}
	for _, obj := range s.Network {
		obj.Save()
	}
}

func (s *Switch) Reload() {
	for _, obj := range s.Network {
		obj.Reload()
	}
}

func (s *Switch) GetNetwork(name string) *Network {
	for _, obj := range s.Network {
		if obj.Name == name {
			return obj
		}
	}
	return nil
}
