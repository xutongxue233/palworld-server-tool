package fleet

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/viper"
)

const (
	ProtocolVersion       = 1
	LocalScope            = "local"
	NodeTokenHeader       = "X-PST-Fleet-Token"
	maxFleetNodes         = 32
	maxFleetNodeIDLength  = 48
	maxFleetNodeName      = 80
	maxFleetNodeURL       = 2048
	minFleetTokenLength   = 32
	maxFleetTokenLength   = 512
	defaultTimeoutSeconds = 15
	minTimeoutSeconds     = 2
	maxTimeoutSeconds     = 120
)

var (
	ErrNodeNotFound      = errors.New("fleet node is not configured")
	ErrNodeConfigInvalid = errors.New("fleet node configuration is invalid")
)

type ConfigIssue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	NodeID  string `json:"node_id,omitempty"`
}

type LocalIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type NodeConfig struct {
	ID                  string        `json:"id"`
	Name                string        `json:"name"`
	BaseURL             string        `json:"base_url"`
	Token               string        `json:"-"`
	AllowPrivateNetwork bool          `json:"allow_private_network"`
	Timeout             time.Duration `json:"-"`
	parsedURL           *url.URL      `json:"-"`
}

type Configuration struct {
	Local  LocalIdentity
	Nodes  []NodeConfig
	Issues []ConfigIssue
}

type nodeConfigInput struct {
	ID                  string `mapstructure:"id"`
	Name                string `mapstructure:"name"`
	BaseURL             string `mapstructure:"base_url"`
	Token               string `mapstructure:"token"`
	AllowPrivateNetwork bool   `mapstructure:"allow_private_network"`
	TimeoutSeconds      int    `mapstructure:"timeout_seconds"`
}

func LoadConfiguration() Configuration {
	configuration := Configuration{
		Local: LocalIdentity{
			ID:   strings.TrimSpace(viper.GetString("fleet.node_id")),
			Name: strings.TrimSpace(viper.GetString("fleet.node_name")),
		},
		Nodes: make([]NodeConfig, 0),
	}
	if !ValidNodeID(configuration.Local.ID) {
		configuration.Issues = append(configuration.Issues, ConfigIssue{
			Code:    "fleet_local_id_invalid",
			Message: "fleet.node_id must use 1-48 lowercase letters, numbers, dashes, or underscores",
			NodeID:  configuration.Local.ID,
		})
		configuration.Local.ID = LocalScope
	}
	if configuration.Local.Name == "" {
		configuration.Local.Name = configuration.Local.ID
	} else if err := validateNodeName(configuration.Local.Name); err != nil {
		configuration.Issues = append(configuration.Issues, ConfigIssue{
			Code:    "fleet_local_name_invalid",
			Message: err.Error(),
			NodeID:  configuration.Local.ID,
		})
		configuration.Local.Name = configuration.Local.ID
	}
	if token := viper.GetString("fleet.node_token"); token != "" && !ValidTokenSecret(token) {
		configuration.Issues = append(configuration.Issues, ConfigIssue{
			Code:    "fleet_node_token_invalid",
			Message: "fleet.node_token must contain 32-512 printable ASCII characters without whitespace",
			NodeID:  configuration.Local.ID,
		})
	}

	var inputs []nodeConfigInput
	if err := viper.UnmarshalKey("fleet.nodes", &inputs); err != nil {
		configuration.Issues = append(configuration.Issues, ConfigIssue{
			Code:    "fleet_nodes_invalid",
			Message: "decode fleet.nodes: " + err.Error(),
		})
		return configuration
	}
	if len(inputs) > maxFleetNodes {
		configuration.Issues = append(configuration.Issues, ConfigIssue{
			Code:    "fleet_nodes_limit",
			Message: fmt.Sprintf("fleet.nodes cannot contain more than %d entries", maxFleetNodes),
		})
		inputs = inputs[:maxFleetNodes]
	}

	seen := map[string]bool{strings.ToLower(LocalScope): true, strings.ToLower(configuration.Local.ID): true}
	defaultTimeout := normalizedFleetTimeout(viper.GetInt("fleet.timeout_seconds"))
	for index, input := range inputs {
		nodeID := strings.TrimSpace(input.ID)
		issuePrefix := fmt.Sprintf("fleet.nodes[%d]", index)
		if !ValidNodeID(nodeID) {
			configuration.Issues = append(configuration.Issues, ConfigIssue{
				Code:    "fleet_node_id_invalid",
				Message: issuePrefix + ".id must use 1-48 lowercase letters, numbers, dashes, or underscores",
				NodeID:  nodeID,
			})
			continue
		}
		foldedID := strings.ToLower(nodeID)
		if seen[foldedID] {
			configuration.Issues = append(configuration.Issues, ConfigIssue{
				Code:    "fleet_node_id_duplicate",
				Message: issuePrefix + ".id duplicates the local identity, reserved local scope, or another node",
				NodeID:  nodeID,
			})
			continue
		}

		name := strings.TrimSpace(input.Name)
		if name == "" {
			name = nodeID
		}
		if err := validateNodeName(name); err != nil {
			configuration.Issues = append(configuration.Issues, ConfigIssue{
				Code:    "fleet_node_name_invalid",
				Message: issuePrefix + ".name: " + err.Error(),
				NodeID:  nodeID,
			})
			continue
		}
		if !ValidTokenSecret(input.Token) {
			configuration.Issues = append(configuration.Issues, ConfigIssue{
				Code:    "fleet_node_token_invalid",
				Message: issuePrefix + ".token must contain 32-512 printable ASCII characters without whitespace",
				NodeID:  nodeID,
			})
			continue
		}
		parsed, canonical, err := parseNodeBaseURL(input.BaseURL)
		if err != nil {
			configuration.Issues = append(configuration.Issues, ConfigIssue{
				Code:    "fleet_node_url_invalid",
				Message: issuePrefix + ".base_url: " + err.Error(),
				NodeID:  nodeID,
			})
			continue
		}
		if parsed.Scheme == "http" && !input.AllowPrivateNetwork {
			configuration.Issues = append(configuration.Issues, ConfigIssue{
				Code:    "fleet_node_http_requires_opt_in",
				Message: issuePrefix + ".base_url uses plain HTTP; set allow_private_network=true only for a trusted private network",
				NodeID:  nodeID,
			})
			continue
		}
		timeout := defaultTimeout
		if input.TimeoutSeconds != 0 {
			if input.TimeoutSeconds < minTimeoutSeconds || input.TimeoutSeconds > maxTimeoutSeconds {
				configuration.Issues = append(configuration.Issues, ConfigIssue{
					Code: "fleet_node_timeout_invalid",
					Message: fmt.Sprintf(
						"%s.timeout_seconds must be between %d and %d",
						issuePrefix,
						minTimeoutSeconds,
						maxTimeoutSeconds,
					),
					NodeID: nodeID,
				})
				continue
			}
			timeout = time.Duration(input.TimeoutSeconds) * time.Second
		}
		seen[foldedID] = true
		configuration.Nodes = append(configuration.Nodes, NodeConfig{
			ID:                  nodeID,
			Name:                name,
			BaseURL:             canonical,
			Token:               input.Token,
			AllowPrivateNetwork: input.AllowPrivateNetwork,
			Timeout:             timeout,
			parsedURL:           parsed,
		})
	}
	return configuration
}

func FindNode(nodeID string) (NodeConfig, error) {
	configuration := LoadConfiguration()
	for _, node := range configuration.Nodes {
		if strings.EqualFold(node.ID, strings.TrimSpace(nodeID)) {
			return node, nil
		}
	}
	for _, issue := range configuration.Issues {
		if strings.EqualFold(issue.NodeID, strings.TrimSpace(nodeID)) {
			return NodeConfig{}, fmt.Errorf("%w: %s", ErrNodeConfigInvalid, issue.Message)
		}
	}
	return NodeConfig{}, ErrNodeNotFound
}

func ValidNodeID(value string) bool {
	if value == "" || len(value) > maxFleetNodeIDLength || value != strings.ToLower(value) {
		return false
	}
	for index, character := range value {
		valid := character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
		if index > 0 && index < len(value)-1 && (character == '-' || character == '_') {
			valid = true
		}
		if !valid {
			return false
		}
	}
	return true
}

func ValidTokenSecret(value string) bool {
	if len(value) < minFleetTokenLength || len(value) > maxFleetTokenLength || value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}

func validateNodeName(value string) error {
	if value == "" || len(value) > maxFleetNodeName || !utf8.ValidString(value) {
		return fmt.Errorf("must be valid UTF-8 text between 1 and %d bytes", maxFleetNodeName)
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return errors.New("cannot contain control characters")
		}
	}
	return nil
}

func parseNodeBaseURL(raw string) (*url.URL, string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > maxFleetNodeURL {
		return nil, "", fmt.Errorf("must be a non-empty URL no longer than %d characters", maxFleetNodeURL)
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, "", err
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, "", errors.New("scheme must be https or explicitly opted-in private HTTP")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return nil, "", errors.New("host is required")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
		return nil, "", errors.New("userinfo, query, fragment, and opaque URLs are not allowed")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return nil, "", errors.New("base_url must be an origin without a path")
	}
	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, "", errors.New("port must be between 1 and 65535")
		}
	}
	parsed.Path = ""
	parsed.RawPath = ""
	canonical := strings.TrimSuffix(parsed.String(), "/")
	return parsed, canonical, nil
}

func normalizedFleetTimeout(seconds int) time.Duration {
	if seconds == 0 {
		seconds = defaultTimeoutSeconds
	}
	if seconds < minTimeoutSeconds {
		seconds = minTimeoutSeconds
	}
	if seconds > maxTimeoutSeconds {
		seconds = maxTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
