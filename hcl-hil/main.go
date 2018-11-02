package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/gopherjs/gopherjs/js"
	"github.com/hashicorp/hcl"
	hclParser "github.com/hashicorp/hcl/hcl/parser"
	hclToken "github.com/hashicorp/hcl/hcl/token"
	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/hashicorp/hil/parser"
	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/config/module"
	"github.com/hashicorp/terraform/dag"
	"github.com/hashicorp/terraform/flatmap"
	"github.com/hashicorp/terraform/terraform"
	"github.com/terraform-providers/terraform-provider-aws/aws"
)

// wrapper struct for terraform.InstanceInfo so we can do some customized behavior
type cytoInstanceInfo struct {
	II *terraform.InstanceInfo
	ID string
}

func newInstanceInfo(ii *terraform.InstanceInfo, index int) *cytoInstanceInfo {
	p := new(cytoInstanceInfo)
	if index == 0 {
		p.ID = ii.HumanId()
	} else {
		p.ID = ii.HumanId() + strconv.Itoa(index)
	}
	p.II = ii
	return p
}

type hclError struct {
	Pos *hclToken.Pos
	Err string
}

func parseHcl(v string) (interface{}, *hclError) {
	result, err := hcl.ParseString(v)

	if err != nil {
		if pErr, ok := err.(*hclParser.PosError); ok {
			return nil, &hclError{
				Pos: &pErr.Pos,
				Err: pErr.Err.Error(),
			}
		}

		return result, &hclError{
			Pos: nil,
			Err: err.Error(),
		}
	}

	return result, nil
}

type hilError struct {
	Pos *ast.Pos
	Err string
}

func parseHilWithPosition(v string, column, line int, filename string) (interface{}, *hilError) {
	result, err := hil.ParseWithPosition(v, ast.Pos{
		Column:   column,
		Line:     line,
		Filename: filename,
	})

	if err != nil {
		if pErr, ok := err.(*parser.ParseError); ok {
			return nil, &hilError{
				Pos: &pErr.Pos,
				Err: pErr.String(),
			}
		}

		return nil, &hilError{
			Pos: nil,
			Err: err.Error(),
		}
	}

	return result, nil
}

type goError struct {
	Err string
}

func readPlan(v []uint8) (interface{}, *goError) {
	reader := bytes.NewReader(v)

	plan, err := terraform.ReadPlan(reader)
	if err != nil {
		return nil, &goError{Err: err.Error()}
	}

	return plan, nil
}

type cytoscapeNodeBody struct {
	ID       string                 `json:"id,omitempty"`
	Name     string                 `json:"name,omitempty"`
	NodeType string                 `json:"type,omitempty"`
	LocalID  string                 `json:"local_id,omitempty"`
	NodeData map[string]interface{} `json:"node_data,omitempty"`
	Parent   string                 `json:"parent,omitempty"`
	Source   string                 `json:"source,omitempty"`
	Target   string                 `json:"target,omitempty"`
}

type cytoscapeNode struct {
	Data cytoscapeNodeBody `json:"data"`
}

func loadJSON(raw string) (interface{}, *goError) {
	load, err := config.LoadJSON([]byte(raw))
	if err != nil {
		return nil, &goError{Err: err.Error()}
	}
	return load, nil
}

func loadDir(path string) (interface{}, *goError) {
	load, err := config.LoadDir(path)
	if err != nil {
		return nil, &goError{Err: err.Error()}
	}
	return load, nil
}
func isInterpolated(field string) bool {
	var validInterpolation = regexp.MustCompile(`^\$\{.*\}$`)
	return validInterpolation.MatchString(field)
}

// take an interpolated variable e.g. - "${foo.bar.id}" and return "foo.bar"
func stripInterpolateSyntax(res *config.Resource, id string) (string, bool) {
	//Check for a valid hcl interpolation ${sample}
	if isInterpolated(id) {
		//Check if variable map exists, be sure to strip out the interpolation tokens ${}
		if v, ok := res.RawConfig.Variables[id[2:len(id)-1]]; ok {
			//Check if the returning variable is type ResourceVariable
			if rv, ok := v.(*config.ResourceVariable); ok {
				return rv.Type + "." + rv.Name, true
			}
		}
	}

	return id, false
}

// take an interpolated variable e.g. - "${foo.bar.id}" and return "foo.bar"
func strip(id string) (out string) {

	println("interpolating: " + id)
	if isInterpolated(id) {
		out = id[2 : len(id)-1]
		out = strings.TrimSuffix(out, ".id")
	} else {
		out = id
	}

	return out
}

func mapIt(res *config.Resource, resMap map[string]string, keyRaw string, valRaw string, loopKey bool, loopVal bool) error {
	key, keyStripped := stripInterpolateSyntax(res, keyRaw)
	val, valStripped := stripInterpolateSyntax(res, valRaw)

	println("mapIt:(enter) key:" + key + " val:" + val)
	count, err := res.Count()
	if err != nil {
		println("mapIt (ERROR): " + err.Error())
		return err
	}
	println("mapIt: count = " + strconv.Itoa(count))
	for i := 0; i < count; i++ {
		index := strconv.Itoa(i)

		switch {
		case loopKey && !loopVal:
			if valStripped {
				resMap[key+"."+index] = val + ".0"
			} else {
				resMap[key+"."+index] = val
			}
		case !loopKey && loopVal:
			if keyStripped {
				resMap[key+".0"] = val + "." + index
			} else {
				resMap[key] = val + "." + index
			}
		case loopKey && loopVal:
			resMap[key+"."+index] = val + "." + index
		case !loopKey && !loopVal:
			resMap[key] = val
			break
		}
	}

	for k, v := range resMap {
		println("mapIt: key:" + k + " val:" + v)
	}
	return nil
}
func mapIt2(resMap map[string]string, keyRaw string, valRaw string) error {
	key := strip(keyRaw)
	val := strip(valRaw)

	println("mapIt2:(enter) key:" + key + " val:" + val)
	resMap[key] = val

	for k, v := range resMap {
		println("mapIt: key:" + k + " val:" + v)
	}
	return nil
}
func mapCidr(res *config.Resource, resMap map[string][]*net.IPNet, keyRaw string, val *net.IPNet) error {
	key, _ := stripInterpolateSyntax(res, keyRaw)

	count, err := res.Count()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		index := strconv.Itoa(i)
		resMap[key+"."+index] = append(resMap[key+"."+index], val)
	}

	return nil
}

func mapCidr2(res *config.Resource, resMap map[*net.IPNet][]string, key *net.IPNet, valRaw string) error {
	val, _ := stripInterpolateSyntax(res, valRaw)

	resMap[key] = append(resMap[key], val)

	return nil
}

func mapMembership(res *config.Resource, resMap map[string][]string, keyRaw string, valRaw string, loopKey bool) error {

	key, keyStripped := stripInterpolateSyntax(res, keyRaw)
	val, valStripped := stripInterpolateSyntax(res, valRaw)

	count, err := res.Count()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		index := strconv.Itoa(i)
		if loopKey {
			if valStripped {
				println("mapMem: adding " + key + "." + index + " = " + val + ".0")
				resMap[key+"."+index] = append(resMap[key+"."+index], val+".0")
			} else {
				println("mapMem: adding " + key + "." + index + " = " + val)
				resMap[key+"."+index] = append(resMap[key+"."+index], val)
			}
		} else {
			if keyStripped {
				println("mapMem: adding " + key + ".0" + " = " + val + "." + index)
				resMap[key+".0"] = append(resMap[key+".0"], val+"."+index)
			} else {
				println("mapMem: adding " + key + " = " + val + "." + index)
				resMap[key] = append(resMap[key], val+"."+index)
			}
		}
	}
	for k, v := range resMap {
		for _, vl := range v {
			println("mapMem: key:" + k + " val:" + vl)
		}
	}
	return nil
}
func mapMembership2(resMap map[string][]string, keyRaw string, valRaw string) error {

	key := strip(keyRaw)
	val := strip(valRaw)

	println("mapMem: adding " + key + " = " + val)
	resMap[key] = append(resMap[key], val)

	return nil
}

//EndPointGroup - represents either a Security Group or a CIDR block
type EndPointGroup struct {
	ID        string
	EndPoints map[string]bool // membership to EndPointGroup.  members can be other EPG's, instances or network interfaces
}

func (epg *EndPointGroup) Hashcode() interface{} {
	return epg.ID
}
func NewEndPointGroup(id string) *EndPointGroup {
	return &EndPointGroup{
		ID:        id,
		EndPoints: map[string]bool{},
	}
}

// prepend name with each modulePath entry.  E.g. - if modulePath = {"root", "base", "sub"} and name = "foo", then return "module.base.sub.foo"
func modulePath(modulePath []string, name string) string {
	if len(modulePath) <= 1 {
		return name
	}
	if strings.HasPrefix(name, "module") {
		return name
	}

	return fmt.Sprintf(
		"module.%s.%s",
		strings.Join(modulePath[1:], "."),
		name)
}

type graph struct {
	CytoscapeData        *[]cytoscapeNode
	ParentMap            map[string]string
	SubNIMembership      map[string][]string
	SgNiMembership       map[string][]string
	SgEc2Membership      map[string][]string
	NiSgMembership       map[string][]string
	NiEc2Map             map[string]string
	SgIngressCidrs       map[string][]*net.IPNet
	SgIngressSgs         map[string][]string
	SgEgressCidrs        map[string][]*net.IPNet
	SgEgressSgs          map[string][]string
	CidrSubnetMembership map[*net.IPNet][]string
	SubCidrMap           map[string]string
	CidrEc2Membership    map[string][]string
	SubEc2Membership     map[string][]string
}

func (g graph) addParent(info *cytoInstanceInfo, parent string) error {
	return mapIt2(g.ParentMap, info.ID, parent)
}
func (g graph) addSubNiMembership(ni *config.Resource, subID string, netID string) error {
	return mapMembership(ni, g.SubNIMembership, subID, netID, false)
}
func (g graph) addSubNiMembership2(subID string, netID string) error {
	return mapMembership2(g.SubNIMembership, subID, netID)
}
func (g graph) addSgNiMembership(ni *config.Resource, sgID string, netID string) error {
	return mapMembership(ni, g.SgNiMembership, sgID, netID, false)
}
func (g graph) addSgNiMembership2(sgID string, netID string) error {
	return mapMembership2(g.SgNiMembership, sgID, netID)
}
func (g graph) addNiSgMembership(ni *config.Resource, netID string, sgID string) error {
	return mapMembership(ni, g.NiSgMembership, netID, sgID, true)
}
func (g graph) addNiSgMembership2(netID string, sgID string) error {
	return mapMembership2(g.NiSgMembership, netID, sgID)
}
func (g graph) addSgEc2Membership(ec2 *config.Resource, sgID string, ec2ID string) error {
	return mapMembership(ec2, g.SgEc2Membership, sgID, ec2ID, false)
}
func (g graph) addSgEc2Membership2(sgID string, ec2ID string) error {
	return mapMembership2(g.SgEc2Membership, sgID, ec2ID)
}
func (g graph) addNiEc2Map(ec2 *config.Resource, netID string, ec2ID string) error {
	return mapIt(ec2, g.NiEc2Map, netID, ec2ID, true, true)
}
func (g graph) addNiEc2Map2(netID string, ec2ID string) error {
	return mapIt2(g.NiEc2Map, netID, ec2ID)
}
func (g graph) addSubCidrMap(subnet string, cidr string) error {
	return mapIt2(g.SubCidrMap, subnet, cidr)
}
func (g graph) addSgIngressCidrs(sg *config.Resource, sgID string, cidr *net.IPNet) error {
	return mapCidr(sg, g.SgIngressCidrs, sgID, cidr)
}
func (g graph) addSgIngSgs(sg *config.Resource, sgID1 string, sgID2 string) error {
	return mapMembership(sg, g.SgIngressSgs, sgID1, sgID2, true)
}
func (g graph) addSgEgrCidrs(sg *config.Resource, sgID string, cidr *net.IPNet) error {
	return mapCidr(sg, g.SgEgressCidrs, sgID, cidr)
}
func (g graph) addSgEgrSgs(sg *config.Resource, sgID1 string, sgID2 string) error {
	return mapMembership(sg, g.SgEgressSgs, sgID1, sgID2, true)
}
func (g graph) addCidrSubnetMembership(subnet *config.Resource, cidr *net.IPNet, subID string) error {
	return mapCidr2(subnet, g.CidrSubnetMembership, cidr, subID)
}
func (g graph) addCidrEc2Membership(cidr string, ec2ID string) error {
	return mapMembership2(g.CidrEc2Membership, cidr, ec2ID)
}
func (g graph) addSubEc2Membership(sub string, ec2ID string) error {
	return mapMembership2(g.SubEc2Membership, sub, ec2ID)
}
func (g graph) addNode(info *cytoInstanceInfo, c *terraform.ResourceConfig, nParent string, index int) error {

	parent := strip(nParent)

	nodeData := make(map[string]interface{})
	switch info.II.Type {
	case "aws_subnet":
		if cidr, ok := c.Get("cidr_block"); ok {
			nodeData["CidrBlock"] = cidr
		}
	}

	node := cytoscapeNode{
		Data: cytoscapeNodeBody{
			ID:       info.ID,
			Name:     info.II.HumanId(),
			NodeType: info.II.Type,
			NodeData: nodeData,
			Parent:   parent,
		},
	}
	println("add node: " + node.Data.ID + " parent=" + node.Data.Parent)
	*g.CytoscapeData = append(*g.CytoscapeData, node)
	if err := g.addParent(info, parent); err != nil {
		return err
	}

	return nil
}
func (g graph) addEdge(source string, target string) error {
	println("add edge: " + source + " -> " + target)
	node := cytoscapeNode{
		Data: cytoscapeNodeBody{
			NodeType: "edge",
			Source:   source,
			Target:   target,
		},
	}
	*g.CytoscapeData = append(*g.CytoscapeData, node)
	return nil
}
func newGraph() *graph {
	parentMap := make(map[string]string)
	subNIMembership := make(map[string][]string)
	sgNiMembership := make(map[string][]string)
	sgEc2Membership := make(map[string][]string)
	niSgMembership := make(map[string][]string)
	niEc2Map := make(map[string]string)
	sgIngressCidrs := make(map[string][]*net.IPNet)
	sgIngressSgs := make(map[string][]string)
	sgEgressCidrs := make(map[string][]*net.IPNet)
	sgEgressSgs := make(map[string][]string)
	cidrSubnetMembership := make(map[*net.IPNet][]string)
	subCidrMap := make(map[string]string)
	cidrEc2Membership := make(map[string][]string)
	subEc2Membership := make(map[string][]string)
	return &graph{&[]cytoscapeNode{}, parentMap, subNIMembership, sgNiMembership, sgEc2Membership, niSgMembership, niEc2Map, sgIngressCidrs, sgIngressSgs, sgEgressCidrs, sgEgressSgs, cidrSubnetMembership, subCidrMap, cidrEc2Membership, subEc2Membership}
}
func mockProvider(prefix string) *terraform.MockResourceProvider {
	p := new(terraform.MockResourceProvider)

	return p
}
func testProvider(prefix string) *terraform.MockResourceProvider {
	p := new(terraform.MockResourceProvider)
	p.RefreshFn = func(info *terraform.InstanceInfo, s *terraform.InstanceState) (*terraform.InstanceState, error) {
		return s, nil
	}
	p.ResourcesReturn = []terraform.ResourceType{
		terraform.ResourceType{
			Name: fmt.Sprintf("%s_instance", prefix),
		},
		terraform.ResourceType{
			Name: fmt.Sprintf("%s_vpc", prefix),
		},
		terraform.ResourceType{
			Name: fmt.Sprintf("%s_resource", prefix),
		},
	}

	return p
}
func mockContext(opts *terraform.ContextOpts) (*terraform.Context, error) {

	// Enable the shadow graph
	opts.Shadow = true

	ctx, err := terraform.NewContext(opts)
	if err != nil {
		return nil, err
	}

	return ctx, nil
}

func interpolateRawConfig(inter *terraform.Interpolater, scope *terraform.InterpolationScope, rc *config.RawConfig) error {
	vs, err := inter.Values(scope, rc.Variables)

	if err != nil {
		return err
	}

	// Do the interpolation
	if err := rc.Interpolate(vs); err != nil {
		return err
	}
	// put resource computed values back to pre-interpolation
	cfg := rc.Config()
	for key, val := range cfg {
		switch typedReplaceVal := val.(type) {
		case string:
			if typedReplaceVal == config.UnknownVariableValue {
				cfg[key] = rc.Raw[key]
				println("key: " + key + "= " + cfg[key].(string) + "\n")
			}
		case []interface{}:
			for i, v := range typedReplaceVal {
				if strVal, ok := v.(string); ok {
					if strVal == config.UnknownVariableValue {
						cfg[key].([]interface{})[i] = rc.Raw[key].([]interface{})[i]
					}
				}
			}
		case map[string]interface{}:
			for k, v := range typedReplaceVal {
				if v == config.UnknownVariableValue {
					cfg[key].(map[string]interface{})[k] = rc.Raw[key].(map[string]interface{})[k]
				}
			}
		case []map[string]interface{}:
			for i, a := range typedReplaceVal {
				for k, v := range a {
					if v == config.UnknownVariableValue {
						cfg[key].([]map[string]interface{})[i][k] = rc.Raw[key].([]map[string]interface{})[i][k]
					}
				}
			}
		default:
			return errors.New("unexpected variable type for key=" + key)
		}
		if val == config.UnknownVariableValue {
			cfg[key] = rc.Raw[key]
			println("key: " + key + " = " + cfg[key].(string) + "\n")
		}
	}
	return nil
}

// generate ResourceAttrDiffs for nested data structures in tests
func testFlatAttrDiffs(k string, i interface{}) map[string]*terraform.ResourceAttrDiff {
	diffs := make(map[string]*terraform.ResourceAttrDiff)
	// check for strings and empty containers first
	switch t := i.(type) {
	case string:
		diffs[k] = &terraform.ResourceAttrDiff{New: t}
		return diffs
	case map[string]interface{}:
		if len(t) == 0 {
			diffs[k] = &terraform.ResourceAttrDiff{New: ""}
			return diffs
		}
	case []interface{}:
		if len(t) == 0 {
			diffs[k] = &terraform.ResourceAttrDiff{New: ""}
			return diffs
		}
	}

	flat := flatmap.Flatten(map[string]interface{}{k: i})

	for k, v := range flat {
		attrDiff := &terraform.ResourceAttrDiff{
			Old: "",
			New: v,
		}
		diffs[k] = attrDiff
	}

	return diffs
}

func evalSG(info *cytoInstanceInfo, c *terraform.ResourceConfig, g *dag.Graph, bIngress bool, tmpG *dag.Graph) error {
	direction := "ingress"
	if !bIngress {
		direction = "egress"
	}
	if rules, ok := c.Get(direction); ok {

		for _, r := range rules.([]map[string]interface{}) {

			if cidrList, ok := r["cidr_blocks"].([]interface{}); ok {
				for _, cidr := range cidrList {
					println("sjl0.1")
					x := strip(cidr.(string))
					var CIDR *net.IPNet
					var err error
					if _, CIDR, err = net.ParseCIDR(x); err != nil {
						return err
					}
					for _, e := range tmpG.Edges() {
						println("check00: " + e.Source().(string) + " -> " + e.Target().(string))
					}
					g.Add(CIDR.String())

					if bIngress {
						println("connecting " + CIDR.String() + " -> " + info.ID)
						tmpG.Connect(dag.BasicEdge(CIDR.String(), info.ID))
					} else {
						println("connecting " + info.ID + " -> " + CIDR.String())
						tmpG.Connect(dag.BasicEdge(info.ID, CIDR.String()))
					}
					for _, e := range tmpG.Edges() {
						println("check0: " + e.Source().(string) + " -> " + e.Target().(string))
					}

					// handle special ingress rule allowing all traffic "0.0.0.0/0", including security groups, even itself
					if CIDR.String() == "0.0.0.0/0" {
						// for all the security groups and cidrs processed so far, add an edge to this security group
						for _, v := range g.Vertices() {
							e := dag.BasicEdge(v.(string), info.ID)
							if !bIngress {
								e = dag.BasicEdge(info.ID, v.(string))
							}

							if g.HasVertex(v) {
								if g.HasEdge(e) {
									if bIngress {
										println("ingress:tmpG.Connect " + v.(string) + " -> " + info.ID)
									} else {
										println("egress:tmpG.Connect " + info.ID + " -> " + v.(string))
									}
									tmpG.Connect(e)
								}
							}
						}
					}
					println("sjl0.4")
				}
			} else if sgList, ok := r["security_groups"].([]interface{}); ok {
				println("sjl0.5")
				for _, sg := range sgList {
					println("sjl0.6")
					SG := strip(sg.(string))
					if bIngress {
						println("tmpG.connecting " + SG + " to " + info.ID)
						tmpG.Connect(dag.BasicEdge(SG, info.ID))
					} else {
						println("tmpG.connecting " + info.ID + " to " + SG)
						tmpG.Connect(dag.BasicEdge(info.ID, SG))
					}
				}
			} else {
				return errors.New("security group ingress rule must have either cidr or security group")
			}
		}
	}

	return nil
}

func connectByCidr(info *cytoInstanceInfo, subnet string, g *dag.Graph, thisGraph *graph) error {
	//Look for any cidr block sg rules that apply this the current instance
	currentCidr, ok := thisGraph.SubCidrMap[subnet]
	if !ok {
		return errors.New("Unexpected error: couldn't match a subnet to its cidr block")
	}
	ip, cCidr, err := net.ParseCIDR(currentCidr)
	if err != nil {
		return err
	}
	cidrSize1, _ := cCidr.Mask.Size()
	for _, v := range g.Vertices() {
		if _, cidr, err := net.ParseCIDR(v.(string)); err == nil {
			//found a CIDR
			println("is " + ip.String() + " in " + cidr.String() + "?")
			// special case: if this instance belongs to a subnet whos CIDR is *larger* than the SG rule's cidr, then *reject* connection
			//               e.g. - if subnet CIDR = 10.0.0.0/23, and the SG rule allows ingress from 10.0.0.0/24 then don't draw the edge
			//                      since there's a 50% chance that this instance would acquire an IP outside the range of 10.0.0.0/24, like
			//						10.1.0.10
			cidrSize2, _ := cidr.Mask.Size()
			println("cidrSize2: " + strconv.Itoa(cidrSize2) + " cidrSize1: " + strconv.Itoa(cidrSize1))
			if cidrSize2 <= cidrSize1 { // the larger the mask size, the smaller the CIDR
				if cidr.Contains(ip) {
					println("yes")
					// instance is a member of this CIDR
					for _, e := range g.UpEdges(cidr.String()).List() {
						//we assume the other end must be a security group
						for _, v := range thisGraph.SgEc2Membership[e.(string)] {
							//draw edge
							thisGraph.addEdge(v, info.ID)
						}

					}
					for _, e := range g.DownEdges(cidr.String()).List() {
						for _, v := range thisGraph.SgEc2Membership[e.(string)] {
							//draw edge
							thisGraph.addEdge(info.ID, v)
						}
					}
				}
			}
		}
	}
	return nil
}
func connectBySG(info *cytoInstanceInfo, sg string, g *dag.Graph, thisGraph *graph) error {
	println("searching sg: " + sg)
	for _, e := range g.UpEdges(sg).List() {
		println("UpEdges to " + e.(string))
		for _, v := range thisGraph.SgEc2Membership[e.(string)] {
			//draw edge
			thisGraph.addEdge(v, info.ID)
		}
	}
	for _, e := range g.DownEdges(sg).List() {
		println("Downedges to " + e.(string))
		for _, v := range thisGraph.SgEc2Membership[e.(string)] {
			//draw edge
			thisGraph.addEdge(info.ID, v)
		}
	}
	thisGraph.addSgEc2Membership2(sg, info.ID)
	return nil
}
func interpolateConfig(m *module.Tree, thisGraph *graph) error {

	p := testProvider("aws")
	var g dag.Graph // network pathing graph

	p.DiffFn = func(
		ii *terraform.InstanceInfo,
		s *terraform.InstanceState,
		c *terraform.ResourceConfig) (*terraform.InstanceDiff, error) {

		println("TYPE: " + ii.Type + "NAME: " + ii.HumanId() + "MODULEPATH: ")
		fmt.Println("RAW:")
		fmt.Println(c.Raw)
		fmt.Println("CONFIG:")
		fmt.Println(c.Config)
		for _, v := range ii.ModulePath {
			println(v)
		}
		info := newInstanceInfo(ii, 0)

		switch info.II.Type {
		case "aws_vpc":
			thisGraph.addNode(info, c, "", 0)
		case "aws_subnet":
			if p, ok := c.Get("vpc_id"); ok {
				// add parent
				vpc := strip(p.(string))
				if err := thisGraph.addNode(info, c, modulePath(ii.ModulePath, vpc), 0); err != nil {
					return nil, err
				}
			}
			if p, ok := c.Get("cidr_block"); ok {
				cidr := strip(p.(string))
				if err := thisGraph.addSubCidrMap(info.ID, cidr); err != nil {
					return nil, err
				}
			}

		case "aws_instance":

			var subnet string // limitation: instance belongs to only one subnet, even though instances can have multiple network interfaces, we only support the primary interface
			var sgs []string
			if p, ok := c.Get("subnet_id"); ok {
				subnet = modulePath(ii.ModulePath, strip(p.(string)))
				if err := thisGraph.addNode(info, c, subnet, 0); err != nil {
					return nil, err
				}
				if _sgs, ok := c.Get("vpc_security_group_ids"); ok {
					for _, sg := range _sgs.([]interface{}) {
						sgs = append(sgs, modulePath(ii.ModulePath, strip(sg.(string))))
					}
				}
			} else if p, ok := c.Get("network_interface"); ok {
				for _, ni := range p.([]map[string]interface{}) {
					if did, ok := ni["device_index"]; ok {
						//limitation: Support only the primary network interface for now
						if did.(int) == 0 {
							println("found device index 0")
							if nid, ok := ni["network_interface_id"]; ok {
								netID := modulePath(ii.ModulePath, strip(nid.(string)))
								if err := thisGraph.addNode(info, c, thisGraph.ParentMap[netID], 0); err != nil {
									return nil, err
								}
								if err := thisGraph.addNiEc2Map2(netID, info.ID); err != nil {
									return nil, err
								}
								// draw network connections
								sgs = thisGraph.NiSgMembership[netID]
								subnet = thisGraph.ParentMap[netID]
							}
						}
					}
				}
			}
			for _, sg := range sgs {
				if err := connectBySG(info, sg, &g, thisGraph); err != nil {
					return nil, err
				}
			}

			//Look for any cidr block sg rules that apply this the current instance
			if err := connectByCidr(info, subnet, &g, thisGraph); err != nil {
				return nil, err
			}
			thisGraph.addSubEc2Membership(subnet, info.ID)

		case "aws_network_interface":
			println("network_interface")
			if p, ok := c.Get("subnet_id"); ok {
				subnet_id := modulePath(ii.ModulePath, strip(p.(string)))
				err := thisGraph.addParent(info, subnet_id)
				if err != nil {
					return nil, err
				}
				err = thisGraph.addSubNiMembership2(subnet_id, info.ID)
				if err != nil {
					return nil, err
				}
			}
			if sgs, ok := c.Get("security_groups"); ok {
				for _, _sg := range sgs.([]interface{}) {
					sg := modulePath(ii.ModulePath, strip(_sg.(string)))
					if err := thisGraph.addSgNiMembership2(sg, info.ID); err != nil {
						return nil, err
					}
					if err := thisGraph.addNiSgMembership2(info.ID, sg); err != nil {
						return nil, err
					}
				}
			}
		case "aws_security_group":
			println("sjl0.0")

			g.Add(info.ID)

			//check for any edges pointing to 0.0.0.0/0 then connect them to this security group (SG)
			for _, e := range g.UpEdges("0.0.0.0/0").List() {
				println("g1.connecting " + e.(string) + " -> " + info.ID)
				g.Connect(dag.BasicEdge(e.(string), info.ID))
			}
			for _, e := range g.DownEdges("0.0.0.0/0").List() {
				println("g2.connecting " + info.ID + " -> " + e.(string))
				g.Connect(dag.BasicEdge(info.ID, e.(string)))
			}
			var tmpG dag.Graph
			if err := evalSG(info, c, &g, true, &tmpG); err != nil {
				return nil, err
			}
			if err := evalSG(info, c, &g, false, &tmpG); err != nil {
				return nil, err
			}
			// at this point (A) g.DownEdges(info.ID) should match with (B) tmpG.DownEdges(info.ID)
			// any differences in A should be pruned such that A is subset of B

			println("sjl1")
			for _, e := range tmpG.Edges() {
				println("check1: " + e.Source().(string) + " -> " + e.Target().(string))
			}
			A := g.DownEdges(info.ID)
			println("sjl2")
			B := tmpG.DownEdges(info.ID)
			println("sjl3")
			PruneSet := A.Difference(B)
			for _, e := range tmpG.Edges() {
				println("check2: " + e.Source().(string) + " -> " + e.Target().(string))
			}
			println("sjl4")
			for _, p := range PruneSet.List() {
				println("pruning " + info.ID + " -> " + p.(string))
				g.RemoveEdge(dag.BasicEdge(info.ID, p.(string)))
			}
			for _, e := range tmpG.Edges() {
				println("check3: " + e.Source().(string) + " -> " + e.Target().(string))
			}
			println("sjl1")
			A = g.UpEdges(info.ID)
			println("sjl2")
			B = tmpG.UpEdges(info.ID)
			println("sjl3")
			for _, e := range tmpG.Edges() {
				println("check4: " + e.Source().(string) + " -> " + e.Target().(string))
			}
			PruneSet = A.Difference(B)
			for _, e := range tmpG.Edges() {
				println("check5: " + e.Source().(string) + " -> " + e.Target().(string))
			}
			println("sjl4")
			for _, p := range PruneSet.List() {
				println("pruning " + p.(string) + " -> " + info.ID)
				g.RemoveEdge(dag.BasicEdge(p.(string), info.ID))
			}
			println("sjl6")
			// add the new edges to the main graph
			for _, e := range tmpG.Edges() {
				println("adding: " + e.Source().(string) + " -> " + e.Target().(string))
				g.Connect(e)
			}
			println("sjl8")

		case "aws_elb":
			// elb can belong to multiple subnets, so that means it can have multiple "parents".  cytoscape doesn't support multiple parents,
			// so we will need clone the elb into multiple versions of itself, one for each subnet it belongs to.
			if p, ok := c.Get("subnets"); ok {
				for i, _sub := range p.([]interface{}) {
					sub := modulePath(ii.ModulePath, strip(_sub.(string)))
					clonedInfo := newInstanceInfo(ii, i)
					if err := thisGraph.addNode(clonedInfo, c, sub, i); err != nil {
						return nil, err
					}

					// process security group to security group connections
					if _sgs, ok := c.Get("security_groups"); ok {
						for _, _sg := range _sgs.([]interface{}) {
							sg := modulePath(ii.ModulePath, strip(_sg.(string)))
							if err := connectBySG(clonedInfo, sg, &g, thisGraph); err != nil {
								return nil, err
							}

						}
					}
					//Look for any cidr block sg rules that apply this the current instance
					if err := connectByCidr(clonedInfo, sub, &g, thisGraph); err != nil {
						return nil, err
					}
					thisGraph.addSubEc2Membership(sub, clonedInfo.ID)
				}
			}

		}

		println("sgGrph=" + g.String())

		// Add computed fields from the actual aws provider

		if strings.HasPrefix(info.II.Type, "aws_") {
			p := aws.Provider()
			diff, err := p.Diff(ii, s, c)
			if err != nil {
				return nil, err
			}
			return diff, nil
		}

		return nil, nil
	}

	input := new(terraform.MockUIInput)
	ctx, err := mockContext(&terraform.ContextOpts{
		Module: m,
		ProviderResolver: terraform.ResourceProviderResolverFixed(
			map[string]terraform.ResourceProviderFactory{
				"aws":    terraform.ResourceProviderFactoryFixed(p),
				"null":   terraform.ResourceProviderFactoryFixed(p),
				"google": terraform.ResourceProviderFactoryFixed(p),
			},
		),
		Parallelism: 1,
		UIInput:     input,
	})
	if err != nil {
		return err
	}
	input.InputFn = func(opts *terraform.InputOpts) (string, error) {
		return "", fmt.Errorf("Try setting a default value")
	}

	if err := ctx.Input(terraform.InputModeVar | terraform.InputModeVarUnset); err != nil {
		return err
	}
	//	logging.SetOutput() // suppress verbose logging that shows up in Developer Tool console screen

	if _, err := ctx.Plan(); err != nil {
		return err
	}

	return nil
}
func configToCytoscape(configuration *config.Config) (string, error) {
	m := module.NewTree("config", configuration)
	return moduleToCytoscape(m)
}
func moduleToCytoscape(mod *module.Tree) (string, error) {
	var err error
	thisGraph := newGraph()

	println("calling interpolateConfig")
	if err := interpolateConfig(mod, thisGraph); err != nil {
		println("interpolateConfig ERROR:" + err.Error())
		return "", err
	}
	println("out of interpolateConfig")

	l := strconv.Itoa(len(*thisGraph.CytoscapeData))
	println("length of cytodata=" + l)
	byteArray, err := json.Marshal(*thisGraph.CytoscapeData)
	if err != nil {
		return "", err
	}
	return string(byteArray), nil
}

func tempDir(d string) (string, error) {

	dir, err := ioutil.TempDir(d, "tf")
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dir); err != nil {
		return "", err
	}

	return dir, nil
}
func dirToCytoscape(dir string) (data string) {

	mod, err := module.NewTreeModule("", dir)
	if err != nil {
		panic(err)
	}

	var tmpDir string
	if tmpDir, err = tempDir(dir); err != nil {
		panic(err)
	}
	s := &module.Storage{
		StorageDir: tmpDir,
		Mode:       module.GetModeGet,
	}
	if err = mod.Load(s); err != nil {
		panic(err)
	}
	if data, err = moduleToCytoscape(mod); err != nil {
		panic(err)
	}
	return data
}
func hclToCytoscape(hcl string) (string, error) {

	//var cytoscapeData []cytoscapeNode

	configuration, err := config.LoadJSON([]byte(hcl))
	if err != nil {
		return "", err
	}

	return configToCytoscape(configuration)
}

const typeInvalid = ast.TypeInvalid
const typeAny = ast.TypeAny
const typeBool = ast.TypeBool
const typeString = ast.TypeString
const typeInt = ast.TypeInt
const typeFloat = ast.TypeFloat
const typeList = ast.TypeList
const typeMap = ast.TypeMap
const typeUnknown = ast.TypeUnknown

func main() {
	exports := js.Module.Get("exports")
	exports.Set("parseHcl", parseHcl)
	exports.Set("parseHil", parseHilWithPosition)
	exports.Set("readPlan", readPlan)
	exports.Set("loadJSON", loadJSON)
	exports.Set("loadDir", loadDir)
	exports.Set("hclToCytoscape", hclToCytoscape)
	exports.Set("dirToCytoscape", dirToCytoscape)
	exports.Set("configToCytoscape", configToCytoscape)
	exports.Set("ast", map[string]interface{}{
		"TYPE_INVALID": typeInvalid,
		"TYPE_ANY":     typeAny,
		"TYPE_BOOL":    typeBool,
		"TYPE_STRING":  typeString,
		"TYPE_INT":     typeInt,
		"TYPE_FLOAT":   typeFloat,
		"TYPE_LIST":    typeList,
		"TYPE_MAP":     typeMap,
		"TYPE_UNKNOWN": typeUnknown,
	})
}
