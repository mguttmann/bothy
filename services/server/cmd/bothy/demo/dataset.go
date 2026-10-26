// SPDX-License-Identifier: Apache-2.0

package demo

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"go.yaml.in/yaml/v3"
)

// The files of a data set directory (16 16.3).
const (
	// DataFile is the data set.
	DataFile = "demo.yaml"
	// SchemaFile is its JSON Schema.
	SchemaFile = "demo.schema.json"
)

// The roles of a demo agent (16 16.4).
const (
	RoleWorker   = "worker"
	RoleReviewer = "reviewer"
	RoleWatcher  = "watcher"
)

// ErrInvalidDataSet is the error of a data set that does not match its schema or whose sections
// name each other wrongly; the message names the place, never a text of the data set.
var ErrInvalidDataSet = errors.New("demo: invalid data set")

// DataSet is the parsed demo.yaml (16 16.3).
type DataSet struct {
	Version int      `yaml:"version"`
	Values  Values   `yaml:"values"`
	People  []Person `yaml:"people"`
	// PriceSource is the public source and the start of validity of every price.
	PriceSource PriceSource `yaml:"price_source"`
	Prices      []Price     `yaml:"prices"`
	Agents      []Agent     `yaml:"agents"`
	Spaces      []Space     `yaml:"spaces"`
	Host        Host        `yaml:"host"`
	Prompts     []Prompt    `yaml:"prompts"`
	Live        []Live      `yaml:"live"`
}

// Values are the named sizes and waits of the demo (brief rule 9).
type Values struct {
	// WarmupPrompts is how many prompts the warm-up posts.
	WarmupPrompts int `yaml:"warmup_prompts"`
	// Pace is the wait between two warm-up prompts.
	Pace Duration `yaml:"pace"`
	// AgentWait bounds the wait until every demo agent is connected.
	AgentWait Duration `yaml:"agent_wait"`
	// HostWait bounds the wait for the answer of the host pairing.
	HostWait Duration `yaml:"host_wait"`
	// AgentTokenTTL is the validity of the API tokens of the agents.
	AgentTokenTTL Duration `yaml:"agent_token_ttl"`
	// WorkerSteps, StepInterval, WatcherInterval, HeartbeatInterval, TokensIn and TokensOut go
	// to the demo agent.
	WorkerSteps       int      `yaml:"worker_steps"`
	StepInterval      Duration `yaml:"step_interval"`
	WatcherInterval   Duration `yaml:"watcher_interval"`
	HeartbeatInterval Duration `yaml:"heartbeat_interval"`
	TokensIn          int64    `yaml:"tokens_in"`
	TokensOut         int64    `yaml:"tokens_out"`
}

// Duration is a time.Duration written as Go duration text ("20s", "3m").
type Duration time.Duration

// UnmarshalYAML parses the text form.
func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return fmt.Errorf("%w: %q is no duration", ErrInvalidDataSet, n.Value)
	}
	*d = Duration(v)
	return nil
}

// Person is one person of the data set; exactly one is the owner.
type Person struct {
	Handle      string `yaml:"handle"`
	DisplayName string `yaml:"display_name"`
	Owner       bool   `yaml:"owner"`
	Locale      string `yaml:"locale"`
	TimeZone    string `yaml:"time_zone"`
}

// PriceSource is the source of the made-up prices: an example.com address and the start of their
// validity, which is also their retrieval time (createPrice needs both, 10).
type PriceSource struct {
	URL       string `yaml:"url"`
	ValidFrom string `yaml:"valid_from"`
}

// Price is one price of a made-up model (10, price.manage).
type Price struct {
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
	Unit        string `yaml:"unit"`
	PriceMicros int64  `yaml:"price_micros"`
	Currency    string `yaml:"currency"`
}

// Agent is one demo agent and its role (16 16.4).
type Agent struct {
	Handle      string `yaml:"handle"`
	DisplayName string `yaml:"display_name"`
	Description string `yaml:"description"`
	Role        string `yaml:"role"`
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
}

// Space is one space with its grants and channels.
type Space struct {
	Name     string    `yaml:"name"`
	Icon     string    `yaml:"icon"`
	Color    string    `yaml:"color"`
	Members  []Member  `yaml:"members"`
	Channels []Channel `yaml:"channels"`
}

// Member is one grant on a space: a person or an agent, the level and the capabilities.
type Member struct {
	Person       string   `yaml:"person"`
	Agent        string   `yaml:"agent"`
	Level        string   `yaml:"level"`
	Capabilities []string `yaml:"capabilities"`
}

// Channel is one channel with its operator and its threads.
type Channel struct {
	Name     string   `yaml:"name"`
	Kind     string   `yaml:"kind"`
	Topic    string   `yaml:"topic"`
	Operator string   `yaml:"operator"`
	Threads  []Thread `yaml:"threads"`
}

// Thread is one thread with its messages; the author of the first message opens it.
type Thread struct {
	Title    string    `yaml:"title"`
	Messages []Message `yaml:"messages"`
}

// Message is one message of a person.
type Message struct {
	Author string `yaml:"author"`
	Text   string `yaml:"text"`
}

// Host is the demo host and the session users of its people.
type Host struct {
	Name         string        `yaml:"name"`
	Reason       string        `yaml:"reason"`
	SessionUsers []SessionUser `yaml:"session_users"`
}

// SessionUser maps a person to an operating system user of the host (ruling T12).
type SessionUser struct {
	Person string `yaml:"person"`
	OSUser string `yaml:"os_user"`
}

// Prompt is a warm-up prompt: a thread in a channel with an operator, with text variants.
type Prompt struct {
	Space   string   `yaml:"space"`
	Channel string   `yaml:"channel"`
	Title   string   `yaml:"title"`
	Texts   []string `yaml:"texts"`
}

// Live is a prompt posted after the warm-up whose run stays open.
type Live struct {
	Space   string `yaml:"space"`
	Channel string `yaml:"channel"`
	Title   string `yaml:"title"`
	Person  string `yaml:"person"`
	Text    string `yaml:"text"`
}

// Load reads demo.yaml of dir, validates it against demo.schema.json of the same directory and
// checks the references between its sections. Errors wrap ErrInvalidDataSet, or name a file that
// cannot be read.
//
// Example:
//
//	ds, err := demo.Load("/usr/share/bothy/demo")
func Load(dir string) (*DataSet, error) {
	raw, err := os.ReadFile(filepath.Join(dir, DataFile)) // #nosec G304 -- the operator's data set directory
	if err != nil {
		return nil, fmt.Errorf("demo: read the data set: %w", err)
	}
	schemaRaw, err := os.ReadFile(filepath.Join(dir, SchemaFile)) // #nosec G304 -- the operator's data set directory
	if err != nil {
		return nil, fmt.Errorf("demo: read the schema: %w", err)
	}
	if err := validateSchema(raw, schemaRaw); err != nil {
		return nil, err
	}
	var ds DataSet
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&ds); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidDataSet, err)
	}
	if err := ds.check(); err != nil {
		return nil, err
	}
	return &ds, nil
}

// validateSchema validates the YAML document raw against the JSON Schema schemaRaw.
func validateSchema(raw, schemaRaw []byte) error {
	var doc any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDataSet, err)
	}
	// Through JSON, so the instance has the types the validator expects (map[string]any, float64).
	j, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDataSet, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(j))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidDataSet, err)
	}
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaRaw))
	if err != nil {
		return fmt.Errorf("demo: the schema is no JSON: %w", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(SchemaFile, schemaDoc); err != nil {
		return fmt.Errorf("demo: the schema: %w", err)
	}
	sch, err := c.Compile(SchemaFile)
	if err != nil {
		return fmt.Errorf("demo: the schema: %w", err)
	}
	if err := sch.Validate(inst); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return fmt.Errorf("%w: %s", ErrInvalidDataSet, firstLeaf(ve))
		}
		return fmt.Errorf("%w: %w", ErrInvalidDataSet, err)
	}
	return nil
}

// firstLeaf names the place of the deepest first cause: the location in the instance and the
// keyword, never the value.
func firstLeaf(ve *jsonschema.ValidationError) string {
	for len(ve.Causes) > 0 {
		ve = ve.Causes[0]
	}
	return fmt.Sprintf("/%s does not match the schema", joinLocation(ve.InstanceLocation))
}

// joinLocation joins the tokens of an instance location with "/".
func joinLocation(loc []string) string {
	out := ""
	for i, t := range loc {
		if i > 0 {
			out += "/"
		}
		out += t
	}
	return out
}

// check holds the rules the schema cannot express: exactly one owner, unique handles and names,
// and every reference to a person, agent, space, channel or model resolves.
func (ds *DataSet) check() error {
	people, err := ds.checkPeople()
	if err != nil {
		return err
	}
	agents, err := ds.checkAgents(people)
	if err != nil {
		return err
	}
	spaces, err := ds.checkSpaces(people, agents)
	if err != nil {
		return err
	}
	for _, u := range ds.Host.SessionUsers {
		if !people[u.Person] {
			return invalid("host: unknown person %s", u.Person)
		}
	}
	if err := ds.checkPrompts(spaces, agents); err != nil {
		return err
	}
	return ds.checkLive(spaces, agents)
}

// invalid returns an error that wraps ErrInvalidDataSet with the formatted place.
func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrInvalidDataSet}, a...)...)
}

// checkPeople checks that the handles are unique and exactly one person is the owner; it returns
// the set of handles.
func (ds *DataSet) checkPeople() (map[string]bool, error) {
	people := map[string]bool{}
	owners := 0
	for _, p := range ds.People {
		if people[p.Handle] {
			return nil, invalid("person %s twice", p.Handle)
		}
		people[p.Handle] = true
		if p.Owner {
			owners++
		}
	}
	if owners != 1 {
		return nil, invalid("exactly one person must be the owner, found %d", owners)
	}
	return people, nil
}

// checkAgents checks that the agent handles are unique, no person's handle and that every agent's
// model has a price; it returns the agents by handle.
func (ds *DataSet) checkAgents(people map[string]bool) (map[string]Agent, error) {
	models := map[string]bool{}
	for _, p := range ds.Prices {
		models[p.Provider+"/"+p.Model] = true
	}
	agents := map[string]Agent{}
	for _, a := range ds.Agents {
		if _, ok := agents[a.Handle]; ok || people[a.Handle] {
			return nil, invalid("agent %s twice or also a person", a.Handle)
		}
		if !models[a.Provider+"/"+a.Model] {
			return nil, invalid("agent %s uses a model without a price", a.Handle)
		}
		agents[a.Handle] = a
	}
	return agents, nil
}

// checkSpaces checks the spaces and returns their channels by space and channel name.
func (ds *DataSet) checkSpaces(people map[string]bool, agents map[string]Agent) (map[string]map[string]Channel, error) {
	spaces := map[string]map[string]Channel{}
	for _, s := range ds.Spaces {
		if _, ok := spaces[s.Name]; ok {
			return nil, invalid("space %s twice", s.Name)
		}
		members, err := checkMembers(s, people, agents)
		if err != nil {
			return nil, err
		}
		channels := map[string]Channel{}
		for _, c := range s.Channels {
			if _, ok := channels[c.Name]; ok {
				return nil, invalid("channel %s twice in space %s", c.Name, s.Name)
			}
			if err := checkChannel(s.Name, c, members, people); err != nil {
				return nil, err
			}
			channels[c.Name] = c
		}
		spaces[s.Name] = channels
	}
	return spaces, nil
}

// checkPrompts checks that every warm-up prompt goes to a channel with a worker operator and that
// a person may trigger in its space.
func (ds *DataSet) checkPrompts(spaces map[string]map[string]Channel, agents map[string]Agent) error {
	for i, p := range ds.Prompts {
		c, err := lookupChannel(spaces, p.Space, p.Channel)
		if err != nil {
			return invalid("prompt %d: %v", i, err)
		}
		if a, ok := agents[c.Operator]; !ok || a.Role != RoleWorker {
			return invalid("prompt %d: the channel needs a worker agent as operator", i)
		}
		if len(ds.Space(p.Space).triggerPeople()) == 0 {
			return invalid("prompt %d: no person has trigger on space %s", i, p.Space)
		}
	}
	if ds.Values.WarmupPrompts > 0 && len(ds.Prompts) == 0 {
		return invalid("values.warmup_prompts needs prompts")
	}
	return nil
}

// checkLive checks that every live prompt goes to a channel with a reviewer or watcher operator,
// posted by a person with trigger on its space.
func (ds *DataSet) checkLive(spaces map[string]map[string]Channel, agents map[string]Agent) error {
	for i, l := range ds.Live {
		c, err := lookupChannel(spaces, l.Space, l.Channel)
		if err != nil {
			return invalid("live %d: %v", i, err)
		}
		if a, ok := agents[c.Operator]; !ok || a.Role == RoleWorker {
			return invalid("live %d: the channel needs a reviewer or watcher agent as operator", i)
		}
		if !slices.Contains(ds.Space(l.Space).triggerPeople(), l.Person) {
			return invalid("live %d: %s has no trigger on space %s", i, l.Person, l.Space)
		}
	}
	return nil
}

// checkMembers checks the members of one space and returns the set of their handles.
func checkMembers(s Space, people map[string]bool, agents map[string]Agent) (map[string]bool, error) {
	members := map[string]bool{}
	for _, m := range s.Members {
		switch {
		case m.Person != "" && !people[m.Person]:
			return nil, invalid("space %s: unknown person %s", s.Name, m.Person)
		case m.Agent != "" && agents[m.Agent].Handle == "":
			return nil, invalid("space %s: unknown agent %s", s.Name, m.Agent)
		}
		members[m.Person+m.Agent] = true
	}
	return members, nil
}

// checkChannel checks that the operator of c is a member of its space and every author a person
// of it.
func checkChannel(space string, c Channel, members, people map[string]bool) error {
	if c.Operator != "" && !members[c.Operator] {
		return invalid("space %s channel %s: the operator is no member of the space", space, c.Name)
	}
	for _, t := range c.Threads {
		for _, m := range t.Messages {
			if !members[m.Author] || !people[m.Author] {
				return invalid("space %s channel %s: author %s is no person of the space", space, c.Name, m.Author)
			}
		}
	}
	return nil
}

// lookupChannel returns the channel name of space.
func lookupChannel(spaces map[string]map[string]Channel, space, name string) (Channel, error) {
	cs, ok := spaces[space]
	if !ok {
		return Channel{}, fmt.Errorf("unknown space %s", space)
	}
	c, ok := cs[name]
	if !ok {
		return Channel{}, fmt.Errorf("unknown channel %s in space %s", name, space)
	}
	return c, nil
}

// Owner returns the person marked owner (check guarantees exactly one).
func (ds *DataSet) Owner() Person {
	for _, p := range ds.People {
		if p.Owner {
			return p
		}
	}
	return Person{}
}

// Space returns the space name, or the zero Space.
func (ds *DataSet) Space(name string) Space {
	for _, s := range ds.Spaces {
		if s.Name == name {
			return s
		}
	}
	return Space{}
}

// Channel returns the channel name of the space, or the zero Channel.
func (s Space) Channel(name string) Channel {
	for _, c := range s.Channels {
		if c.Name == name {
			return c
		}
	}
	return Channel{}
}

// triggerPeople returns the handles of the people with level trigger on the space, in the order of
// the data set.
func (s Space) triggerPeople() []string {
	var out []string
	for _, m := range s.Members {
		if m.Person != "" && m.Level == "trigger" {
			out = append(out, m.Person)
		}
	}
	return out
}
