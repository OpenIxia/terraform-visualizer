package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"regexp"
	"strconv"

	"github.com/gopherjs/gopherjs/js"
	"github.com/hashicorp/hcl"
	hclParser "github.com/hashicorp/hcl/hcl/parser"
	hclToken "github.com/hashicorp/hcl/hcl/token"
	"github.com/hashicorp/hil"
	"github.com/hashicorp/hil/ast"
	"github.com/hashicorp/hil/parser"
	"github.com/hashicorp/terraform/config"
	"github.com/hashicorp/terraform/config/module"
	"github.com/hashicorp/terraform/terraform"
)

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
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	NodeType string `json:"type,omitempty"`
	LocalID  string `json:"local_id,omitempty"`
	NodeData []byte `json:"node_data,omitempty"`
	Parent   string `json:"parent,omitempty"`
	Source   string `json:"source,omitempty"`
	Target   string `json:"target,omitempty"`
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
}

func (g graph) addParent(res *config.Resource, parent string) error {
	return mapIt(res, g.ParentMap, res.Id(), parent, true, false)
}
func (g graph) addSubNiMembership(ni *config.Resource, subID string, netID string) error {
	return mapMembership(ni, g.SubNIMembership, subID, netID, false)
}
func (g graph) addSgNiMembership(ni *config.Resource, sgID string, netID string) error {
	return mapMembership(ni, g.SgNiMembership, sgID, netID, false)
}
func (g graph) addNiSgMembership(ni *config.Resource, netID string, sgID string) error {
	return mapMembership(ni, g.NiSgMembership, netID, sgID, true)
}
func (g graph) addSgEc2Membership(ec2 *config.Resource, sgID string, ec2ID string) error {
	return mapMembership(ec2, g.SgEc2Membership, sgID, ec2ID, false)
}
func (g graph) addNiEc2Map(ec2 *config.Resource, netID string, ec2ID string) error {
	return mapIt(ec2, g.NiEc2Map, netID, ec2ID, true, true)
}
func (g graph) addSgIngCidrs(sg *config.Resource, sgID string, cidr *net.IPNet) error {
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
func (g graph) addNode(res *config.Resource, nType string, nParent string) error {
	count, err := res.Count()
	if err != nil {
		return err
	}
	for i := 0; i < count; i++ {
		index := strconv.Itoa(i)
		node := cytoscapeNode{
			Data: cytoscapeNodeBody{
				ID:       res.Id() + "." + index,
				Name:     res.Name,
				NodeType: nType,
				Parent:   nParent,
			},
		}
		println("add node: " + node.Data.ID + " " + node.Data.Name)
		*g.CytoscapeData = append(*g.CytoscapeData, node)
		l := strconv.Itoa(len(*g.CytoscapeData))
		println("length of cytodata=" + l)
	}
	return nil
}
func (g graph) addEdge(source string, target string) error {
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
	return &graph{&[]cytoscapeNode{}, parentMap, subNIMembership, sgNiMembership, sgEc2Membership, niSgMembership, niEc2Map, sgIngressCidrs, sgIngressSgs, sgEgressCidrs, sgEgressSgs, cidrSubnetMembership}
}
func mockProvider(prefix string) *terraform.MockResourceProvider {
	p := new(terraform.MockResourceProvider)

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
func interpolateConfig(c *config.Config) error {

	//p := mockProvider("aws")
	//	m := testModule(t, "validate-good")

	m := module.NewTree("config", c)

	ctx, err := mockContext(&terraform.ContextOpts{
		Module: m,
		//		ProviderResolver: terraform.ResourceProviderResolverFixed(
		//			map[string]terraform.ResourceProviderFactory{
		//				"aws": terraform.ResourceProviderFactoryFixed(p),
		//			},
		//		),
		//		State: state,
	})

	if err != nil {
		return err
	}

	inter := ctx.Interpolater()
	inter.Operation = 6 //terraform.walkValidate

	scope := &terraform.InterpolationScope{
		Path: []string{"root"},
	}

	for _, rez := range m.Config().Resources {

		if err := interpolateRawConfig(inter, scope, rez.RawConfig); err != nil {
			return err
		}
		if err := interpolateRawConfig(inter, scope, rez.RawCount); err != nil {
			return err
		}

	}
	return nil
}
func configToCytoscape(configuration *config.Config) (string, error) {
	var err error
	println("calling interpolateConfig")
	if err := interpolateConfig(configuration); err != nil {
		println("interpolateConfig ERROR:" + err.Error())
		return "", err
	}
	println("out of interpolateConfig")
	thisGraph := newGraph()
	/*
		parentMap := make(map[string]string)
		// map of Network Interface IDs to EC2 instance
		niEc2Map := make(map[string]string)
		// map of Subnet IDs to list of Network Interface IDs
		subNiMembership := make(map[string][]string)
		// map of Network Interface IDs to list of Security Groups
		niSgMembership := make(map[string][]string)
		// map of Security Group IDs to list of instances
		sgEc2Membership := make(map[string][]string)
		// map of Security Grou IDs to list of network interfaces
		sgNiMembership := make(map[string][]string)
		// map of CIDRs to list of instances
		//	cidrEc2Membership := make(map[string][]string)

		// map of Security Group IDs to CIDR or other Security Groups
		sgIngressCidrs := make(map[string][]*net.IPNet)
		sgIngressSg := make(map[string][]string)
		sgEgressCidrs := make(map[string][]*net.IPNet)
		sgEgressSg := make(map[string][]string)
	*/
	println("Pass 1")
	// PASS 1: populate the parent map, that is, given a resource id, store the parent resource id
	for _, resource := range configuration.Resources {
		cfg := resource.RawConfig.Config()
		switch resource.Type {
		case "aws_subnet":
			if p, ok := cfg["vpc_id"]; ok {
				// add parent
				err := thisGraph.addParent(resource, p.(string))
				if err != nil {
					return "", err
				}
			}
		case "aws_network_interface":
			if p, ok := cfg["subnet_id"]; ok {
				// add parent
				err := thisGraph.addParent(resource, p.(string))
				if err != nil {
					return "", err
				}
				err = thisGraph.addSubNiMembership(resource, p.(string), resource.Id())
				if err != nil {
					return "", err
				}
			}
			if sgs, ok := cfg["security_groups"]; ok {
				for _, sg := range sgs.([]interface{}) {
					err = thisGraph.addSgNiMembership(resource, sg.(string), resource.Id())
					if err != nil {
						return "", err
					}
					err = thisGraph.addNiSgMembership(resource, resource.Id(), sg.(string))
					if err != nil {
						return "", err
					}
					/*
						sgID, err := stripInterpolateSyntax(resource, sg.(string))
						if err != nil {
							return "", err
						}
						sgNiMembership[sgID] = append(sgNiMembership[sgID], resource.Id())
						niSgMembership[resource.Id()] = append(niSgMembership[resource.Id()], sgID)
					*/
				}
			}
		case "aws_instance":
			if p, ok := cfg["subnet_id"]; ok {
				err := thisGraph.addParent(resource, p.(string))
				if err != nil {
					return "", err
				}
				// This instance isn't using `network_interface` argument, yet we still need something to identify its
				// primary network interface, so we will use its instance ID
				err = thisGraph.addNiEc2Map(resource, resource.Id(), resource.Id())
				if err != nil {
					return "", err
				}
				err = thisGraph.addSubNiMembership(resource, p.(string), resource.Id())
				if err != nil {
					return "", err
				}

				//niEc2Map[resource.Id()] = resource.Id()
				//subNiMembership[subnetID] = append(subNiMembership[subnetID], resource.Id())
				if sgs, ok := cfg["vpc_security_group_ids"]; ok {
					for _, sg := range sgs.([]interface{}) {
						err := thisGraph.addSgEc2Membership(resource, sg.(string), resource.Id())
						if err != nil {
							return "", err
						}
						/*
							sgID, err := stripInterpolateSyntax(resource, sg.(string))
							if err != nil {
								return "", err
							}
							println("vpc_sg: %s => ec2: %s", sgID, resource.Id())
							sgEc2Membership[sgID] = append(sgEc2Membership[sgID], resource.Id())
						*/
					}
				}
			} else if p, ok := cfg["network_interface"]; ok {
				println("network_interface")
				for _, ni := range p.([]map[string]interface{}) {
					println("found one")
					if did, ok := ni["device_index"]; ok {
						println("found device index")
						//limitation: Support only the primary network interface for now
						if did.(int) == 0 {
							println("found device index 0")
							if nid, ok := ni["network_interface_id"]; ok {

								err := thisGraph.addNiEc2Map(resource, nid.(string), resource.Id())
								if err != nil {
									return "", err
								}
								//// add parent
								//netID, err := stripInterpolateSyntax(resource, nid.(string))
								//if err != nil {
								//	return "", err
								//}
								//println("nid: %s => ec2: %s", netID, resource.Id())
								//niEc2Map[netID] = resource.Id()
							}
						}
					}
				}
				println("exit network_interface")
			}
		case "aws_security_group":
			println("enter aws_security_group: %s", resource.Id()+".0")
			// For this security group (sg) enumerate all the ingress CIDR networks
			if ingressRules, ok := cfg["ingress"].([]map[string]interface{}); ok {
				for _, ingress := range ingressRules {
					println("sg 1")
					// TODO what if cidr_blocks wasn't a literal string, but a variable?
					if cidrList, ok := ingress["cidr_blocks"].([]interface{}); ok {
						for _, cidr := range cidrList {
							println("sg 2 %s", cidr.(string))
							_, n, err := net.ParseCIDR(cidr.(string))
							if err == nil {
								err := thisGraph.addSgIngCidrs(resource, resource.Id(), n)
								if err != nil {
									return "", err
								}
								//sgIngressCidrs[resource.Id()] = append(sgIngressCidrs[resource.Id()], n)
							}
						}
					} else if sgList, ok := ingress["security_groups"].([]interface{}); ok {
						for _, sg := range sgList {
							err := thisGraph.addSgIngSgs(resource, resource.Id(), sg.(string))
							if err != nil {
								return "", err
							}
							//sgID, err := stripInterpolateSyntax(resource, sg.(string))
							//if err != nil {
							//	return "", err
							//}
							//println("sg 2.1 %s", sgID)
							//sgIngressSg[resource.Id()] = append(sgIngressSg[resource.Id()], sgID)
						}
					} else {
						return "", errors.New("security group egress rule must have either cidr or security group")
					}
				}
			}
			// For this security group (sg) enumerate all the egress CIDR networks
			if egressRules, ok := cfg["egress"].([]map[string]interface{}); ok {
				for _, egress := range egressRules {
					println("sg 3")
					if cidrList, ok := egress["cidr_blocks"].([]interface{}); ok {
						for _, cidr := range cidrList {
							println("sg 4 %s", cidr.(string))
							_, n, err := net.ParseCIDR(cidr.(string))
							if err == nil {
								err := thisGraph.addSgEgrCidrs(resource, resource.Id(), n)
								if err != nil {
									return "", err
								}
								//sgEgressCidrs[resource.Id()] = append(sgEgressCidrs[resource.Id()], n)
							}
						}
					} else if sgList, ok := egress["security_groups"].([]interface{}); ok {
						for _, sg := range sgList {
							err := thisGraph.addSgIngSgs(resource, resource.Id(), sg.(string))
							if err != nil {
								return "", err
							}
							//sgID, err := stripInterpolateSyntax(resource, sg.(string))
							//if err != nil {
							//	return "", err
							//}
							//println("sg 4 %s", sgID)
							//if err == nil {
							//	sgEgressSg[resource.Id()] = append(sgEgressSg[resource.Id()], sgID)
							//}
						}
					} else {
						return "", errors.New("security group egress rule must have either cidr or security group")
					}
				}
			}

			println("exit aws_security_group")

		}
	}

	//	cidrSubnetMembership := make(map[*net.IPNet][]string)
	//	subnets := make(map[string]*net.IPNet)
	println("Pass 2")
	for _, resource := range configuration.Resources {
		cfg := resource.RawConfig.Config()
		//		var cn *cytoscapeNode
		switch resource.Type {
		case "aws_vpc":
			thisGraph.addNode(resource, "vpc", "")
			//			cn = &cytoscapeNode{
			//				Data: cytoscapeNodeBody{
			//					ID:       resource.Id(),
			//					Name:     resource.Name,
			//					NodeType: "vpc",
			//					LocalID:  "",
			//				},
			//			}
		case "aws_subnet":
			count, err := resource.Count()
			if err != nil {
				return "", err
			}

			for i := 0; i < count; i++ {
				index := strconv.Itoa(i)
				//println("aws_subnet: " + resource.Id() + " count:" + index)
				if parent, ok := thisGraph.ParentMap[resource.Id()+"."+index]; ok {
					thisGraph.addNode(resource, "subnet", parent)
					//				cn = &cytoscapeNode{
					//					Data: cytoscapeNodeBody{
					//						ID:       resource.Id(),
					//						Name:     resource.Name,
					//						NodeType: "subnet",
					//						LocalID:  "",
					//						Parent:   parent,
					//					},
					//				}
				}
			}
			if c, ok := cfg["cidr_block"]; ok {
				_, ipnet, err := net.ParseCIDR(c.(string))
				if err == nil {

					for _, cidrs := range thisGraph.SgIngressCidrs {
						for _, cidr := range cidrs {
							if cidr.Contains(ipnet.IP) {
								err := thisGraph.addCidrSubnetMembership(resource, cidr, resource.Id()+".0")
								if err != nil {
									return "", err
								}
								//cidrSubnetMembership[cidr] = append(cidrSubnetMembership[cidr], resource.Id())
							}
						}
					}
				}
			}
		case "aws_instance":
			println("aws_instance")
			if p, ok := cfg["network_interface"]; ok {
				println("network_interface: " + resource.Id())
				for _, ni := range p.([]map[string]interface{}) {
					println("found network_interface")
					if did, ok := ni["device_index"]; ok {
						//limitation: Support only the primary network interface for now
						if did.(int) == 0 {
							println("primary interface")
							if nid, ok := ni["network_interface_id"]; ok {
								netID, _ := stripInterpolateSyntax(resource, nid.(string))
								println("nid of primary: " + netID)
								println("sub of primary=" + thisGraph.ParentMap[netID+".0"])
								err := thisGraph.addParent(resource, thisGraph.ParentMap[netID+".0"])
								if err != nil {
									return "", err
								}
								//parentMap[resource.Id()] = parentMap[netID]
								// add security group membership
								if sgs, ok := thisGraph.NiSgMembership[netID+".0"]; ok {
									for _, sg := range sgs {
										println("sg: %s => ec2: %s", sg, resource.Id()+".0")
										err := thisGraph.addSgEc2Membership(resource, sg, resource.Id())
										if err != nil {
											return "", err
										}
										//sgEc2Membership[sg] = append(sgEc2Membership[sg], resource.Id())
									}
								}
							}
						}
					}
				}
			}
			if parent, ok := thisGraph.ParentMap[resource.Id()+".0"]; ok {
				thisGraph.addNode(resource, "ec2", parent)
				//			cn = &cytoscapeNode{
				//				Data: cytoscapeNodeBody{
				//					ID:       resource.Id(),
				//					Name:     resource.Name,
				//					NodeType: "ec2",
				//					LocalID:  "",
				//					Parent:   parent,
				//				},
				//			}
			}
			println("exit aws_instance")
		case "aws_security_group":
			println("enter aws_security_group")
			// TODO: build edges

			println("exit aws_security_group")
		}

	}
	println("Pass 3")
	// Pass 3 is about building the edges between the nodes
	for _, resource := range configuration.Resources {

		//var cn *cytoscapeNode
		switch resource.Type {

		case "aws_security_group":
			println("enter aws_security_group: %s", resource.Id())
			for _, cidr := range thisGraph.SgIngressCidrs[resource.Id()+".0"] {
				println("cidr: %s", cidr.String())
				for _, sub := range thisGraph.CidrSubnetMembership[cidr] {
					println("sub: %s", sub)
					for _, ni := range thisGraph.SubNIMembership[sub] {
						println("ni: %s", ni)
						for _, ec2 := range thisGraph.SgEc2Membership[resource.Id()+".0"] {
							println("ec2: %s", ec2)
							println("santity: %s", thisGraph.NiEc2Map["aws_network_interface.web"])
							println("santity2: %s", thisGraph.NiEc2Map[ni])
							if thisGraph.NiEc2Map[ni] != "" && thisGraph.NiEc2Map[ni] != ec2 {
								println("adding edge source: %s target: %s", thisGraph.NiEc2Map[ni], ec2)
								thisGraph.addEdge(thisGraph.NiEc2Map[ni], ec2)
								//	cn = &cytoscapeNode{
								//		Data: cytoscapeNodeBody{
								//			NodeType: "edge",
								//			Source:   thisGraph.NiEc2Map[ni],
								//			Target:   ec2,
								//		},
								//	}
								//	if cn != nil {
								//		cytoscapeData = append(cytoscapeData, *cn)
								//	}

							}
						}
					}
				}
			}
			for _, sg := range thisGraph.SgIngressSgs[resource.Id()+".0"] {
				println("sg: %s", sg)
				for _, ec2Source := range thisGraph.SgEc2Membership[sg] {
					println("ec2Source: %s", ec2Source)
					for _, ec2Target := range thisGraph.SgEc2Membership[resource.Id()+".0"] {
						println("ec2Target: %s", ec2Target)
						if ec2Source != ec2Target {
							println("adding edge source: %s target: %s", ec2Source, ec2Target)
							thisGraph.addEdge(ec2Source, ec2Target)
							//cn = &cytoscapeNode{
							//	Data: cytoscapeNodeBody{
							//		NodeType: "edge",
							//		Source:   ec2Source,
							//		Target:   ec2Target,
							//	},
							//}
							//if cn != nil {
							//	cytoscapeData = append(cytoscapeData, *cn)
							//}

						}
					}
				}
			}
			println("exit aws_security_group")
		}

	}
	l := strconv.Itoa(len(*thisGraph.CytoscapeData))
	println("length of cytodata=" + l)
	byteArray, err := json.Marshal(*thisGraph.CytoscapeData)
	if err != nil {
		return "", err
	}
	return string(byteArray), nil
}
func dirToCytoscape(dir string) (string, error) {

	configuration, err := config.LoadDir(dir)
	if err != nil {
		return "", err
	}
	return configToCytoscape(configuration)
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
