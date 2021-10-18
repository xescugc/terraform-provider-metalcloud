package metalcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	mc "github.com/metalsoft-io/metal-cloud-sdk-go/v2"
)

func resourceInstanceArray() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceInstanceArrayCreate,
		ReadContext:   resourceInstanceArrayRead,
		UpdateContext: resourceInstanceArrayUpdate,
		DeleteContext: resourceInstanceArrayDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"infrastructure_id": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},
			"instance_array_id": &schema.Schema{
				Type:     schema.TypeInt,
				Computed: true,
			},
			"instance_array_label": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				//this is required because on the serverside the labels are converted to lowercase automatically
				ValidateDiagFunc: validateLabel,
			},
			"instance_array_instance_count": &schema.Schema{
				Type:             schema.TypeInt,
				Optional:         true,
				Default:          nil,
				ValidateDiagFunc: validateMaxOne,
			},
			"instance_array_boot_method": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "pxe_iscsi",
			},
			"instance_array_ram_gbytes": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
			},
			"instance_array_processor_count": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
			},
			"instance_array_processor_core_mhz": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
			},
			"instance_array_processor_core_count": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
			},
			"instance_array_disk_count": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
			},
			"instance_array_disk_size_mbytes": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
			},
			"instance_array_additional_wan_ipv4_json": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"instance_array_custom_variables": {
				Type:     schema.TypeMap,
				Elem:     schema.TypeString,
				Optional: true,
			},
			"instance_custom_variables": {
				Type:     schema.TypeList,
				Elem:     instanceCustomVariableResource(),
				Optional: true,
			},
			"volume_template_id": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
			"instance_array_firewall_managed": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"firewall_rule": {
				Type:     schema.TypeSet,
				Optional: true, //default is computed serverside
				Default:  nil,  //default is computed serverside
				Computed: true, //default is computed serverside
				Elem:     resourceFirewallRule(),
				//Set:      firewallRuleResourceHash,
			},

			"interface": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     resourceInstanceArrayInterface(),
				Set:      interfaceResourceHash,
				//TODO: set defaults so that we don't get the big list of serverside generated rules
			},

			"instances": {
				Type:     schema.TypeString,
				Computed: true,
				Optional: false,
				Default:  nil,
			},
		},
	}
}

func instanceCustomVariableResource() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"instance_index": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
			},
			"custom_variables": &schema.Schema{
				Type:     schema.TypeMap,
				Elem:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceFirewallRule() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"firewall_rule_description": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
			},
			"firewall_rule_port_range_start": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				//Default:  1,
			},
			"firewall_rule_port_range_end": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				//Default:  65535,
			},
			"firewall_rule_source_ip_address_range_start": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			"firewall_rule_source_ip_address_range_end": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			"firewall_rule_destination_ip_address_range_start": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			"firewall_rule_destination_ip_address_range_end": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  nil,
			},
			"firewall_rule_protocol": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "tcp",
			},
			"firewall_rule_ip_address_type": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Default:  "ipv4",
			},
			"firewall_rule_enabled": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
		},
	}
}

func resourceInstanceArrayInterface() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"interface_index": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"network_label": {
				Type:     schema.TypeString,
				Required: true,
			},
			"network_id": {
				Type:     schema.TypeInt,
				Optional: true,
				Computed: true,
			},
		},
	}
}

func resourceInstanceArrayCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	client := meta.(*mc.Client)

	infrastructure_id := d.Get("infrastructure_id").(int)

	ia := expandInstanceArray(d)

	iaC, err := client.InstanceArrayCreate(infrastructure_id, ia)
	if err != nil {
		return diag.FromErr(err)
	}

	id := fmt.Sprintf("%d", iaC.InstanceArrayID)

	d.SetId(id)

	return resourceInstanceArrayRead(ctx, d, meta)
}

func resourceInstanceArrayRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	var diags diag.Diagnostics

	client := meta.(*mc.Client)

	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	ia, err := client.InstanceArrayGet(id)
	if err != nil {
		return diag.FromErr(err)
	}

	flattenInstanceArray(d, *ia)

	/* INSTANCES */
	retInstances, err := client.InstanceArrayInstances(ia.InstanceArrayID)
	if err != nil {
		return diag.FromErr(err)
	}

	retInstancesJSON, err := flattenInstances(retInstances)
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("instances", retInstancesJSON)

	/* INSTANCES CUSTOM VARS */
	instancesCustomVariables := flattenInstancesCustomVariables(retInstances)

	if len(instancesCustomVariables) > 0 {
		d.Set("instance_custom_variables", instancesCustomVariables)

	}

	//TODO: interfaces

	return diags

}

func flattenInstances(val interface{}) (string, error) {

	retInstances := val.(*map[string]mc.Instance)

	instanceMap := make(map[int]mc.Instance, len(*retInstances))
	keys := []int{}
	instances := []mc.Instance{}

	for _, v := range *retInstances {
		instanceMap[v.InstanceID] = v
		keys = append(keys, v.InstanceID)
	}

	sort.Ints(keys)

	for _, id := range keys {
		instances = append(instances, instanceMap[id])
	}

	bytes, err := json.Marshal(instances)
	if err != nil {
		return "", fmt.Errorf("error serializing instances array: %s", err)
	}

	return string(bytes), nil
}

func resourceInstanceArrayUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	client := meta.(*mc.Client)

	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	retIA, err := client.InstanceArrayGet(id)
	if err != nil {
		return diag.FromErr(err)
	}

	ia := expandInstanceArray(d)

	//update interface operations
	for _, intf := range ia.InstanceArrayInterfaces {
		for _, opIntf := range retIA.InstanceArrayOperation.InstanceArrayInterfaces {
			if opIntf.InstanceArrayInterfaceID == intf.InstanceArrayInterfaceID {
				copyInstanceArrayInterfaceToOperation(intf, &opIntf)
			}
		}
	}

	//update the main operation object
	copyInstanceArrayToOperation(ia, retIA.InstanceArrayOperation)

	bSwapExistingInstancesHardware := false
	bkeepDetachingDrives := false

	_, err = client.InstanceArrayEdit(id, *retIA.InstanceArrayOperation, &bSwapExistingInstancesHardware, &bkeepDetachingDrives, nil, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	/* custom variables for instances */
	cvList := d.Get("instance_custom_variables").([]interface{})
	diag := updateInstancesCustomVariables(cvList, id, client)

	if diag.HasError() {
		return diag
	}

	//TODO: handle interfaces
	return resourceInstanceArrayRead(ctx, d, meta)
}

func resourceInstanceArrayDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	var diags diag.Diagnostics
	client := meta.(*mc.Client)

	id, err := strconv.Atoi(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	err = client.InstanceArrayDelete(id)
	d.SetId("")
	return diags
}

func interfaceToString(v interface{}) string {
	var buf bytes.Buffer

	i := v.(map[string]interface{})

	// instance_array_interface_label := i["instance_array_interface_label"].(string)
	// instance_array_interface_service_status := i["instance_array_interface_service_status"].(string)
	instance_array_interface_index := strconv.Itoa(i["interface_index"].(int))
	network_id := strconv.Itoa(i["network_id"].(int))
	network_label := i["network_label"].(string)

	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(network_label)))
	// buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(instance_array_interface_service_status)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(instance_array_interface_index)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(network_id)))

	return buf.String()
}

func interfaceResourceHash(v interface{}) int {
	return hash(interfaceToString(v))
}

func firewallRuleResourceHash(v interface{}) int {
	var buf bytes.Buffer
	fr := v.(map[string]interface{})

	firewall_rule_description := fr["firewall_rule_description"].(string)
	firewall_rule_source_ip_address_range_start := fr["firewall_rule_source_ip_address_range_start"].(string)
	firewall_rule_source_ip_address_range_end := fr["firewall_rule_source_ip_address_range_end"].(string)
	firewall_rule_destination_ip_address_range_start := fr["firewall_rule_destination_ip_address_range_start"].(string)
	firewall_rule_destination_ip_address_range_end := fr["firewall_rule_destination_ip_address_range_end"].(string)
	firewall_rule_protocol := fr["firewall_rule_protocol"].(string)
	firewall_rule_ip_address_type := fr["firewall_rule_ip_address_type"].(string)
	firewall_rule_port_range_start := strconv.Itoa(fr["firewall_rule_port_range_start"].(int))
	firewall_rule_port_range_end := strconv.Itoa(fr["firewall_rule_port_range_end"].(int))
	firewall_rule_enabled := strconv.FormatBool(fr["firewall_rule_enabled"].(bool))

	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_description)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_source_ip_address_range_start)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_source_ip_address_range_end)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_destination_ip_address_range_start)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_destination_ip_address_range_end)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_protocol)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_ip_address_type)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_port_range_start)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_port_range_end)))
	buf.WriteString(fmt.Sprintf("%s-", strings.ToLower(firewall_rule_enabled)))

	return hash(buf.String())
}

func hash(v string) int {
	hash := crc32.ChecksumIEEE([]byte(v))

	return int(hash)

}

func flattenInstanceArray(d *schema.ResourceData, instanceArray mc.InstanceArray) error {

	d.Set("instance_array_id", instanceArray.InstanceArrayID)
	d.Set("instance_array_label", instanceArray.InstanceArrayLabel)
	d.Set("instance_array_instance_count", instanceArray.InstanceArrayInstanceCount)
	d.Set("instance_array_boot_method", instanceArray.InstanceArrayBootMethod)
	d.Set("instance_array_ram_gbytes", instanceArray.InstanceArrayRAMGbytes)
	d.Set("instance_array_processor_count", instanceArray.InstanceArrayProcessorCount)
	d.Set("instance_array_processor_core_mhz", instanceArray.InstanceArrayProcessorCoreMHZ)
	d.Set("instance_array_processor_core_count", instanceArray.InstanceArrayProcessorCoreCount)
	d.Set("instance_array_disk_count", instanceArray.InstanceArrayDiskCount)
	d.Set("instance_array_disk_size_mbytes", instanceArray.InstanceArrayDiskSizeMBytes)
	d.Set("volume_template_id", instanceArray.VolumeTemplateID)
	d.Set("instance_array_firewall_managed", instanceArray.InstanceArrayFirewallManaged)
	d.Set("instance_array_additional_wan_ipv4_json", instanceArray.InstanceArrayAdditionalWanIPv4JSON)

	/* INSTANCE ARRAY CUSTOM VARIABLES */
	switch instanceArray.InstanceArrayCustomVariables.(type) {
	case []interface{}:
		d.Set("instance_array_custom_variables", make(map[string]string))
	default:
		iacv := make(map[string]string)

		for k, v := range instanceArray.InstanceArrayCustomVariables.(map[string]interface{}) {
			iacv[k] = v.(string)
		}
		d.Set("instance_array_custom_variables", iacv)
	}

	/* FIREWALL RULES */
	fwRules := []interface{}{}

	for _, fw := range instanceArray.InstanceArrayFirewallRules {
		fwRules = append(fwRules, flattenFirewallRule(fw))
	}
	if len(fwRules) > 0 {
		d.Set("firewall_rule", schema.NewSet(schema.HashResource(resourceFirewallRule()), fwRules))
	}

	return nil
}

func expandInstanceArray(d *schema.ResourceData) mc.InstanceArray {

	var ia mc.InstanceArray
	if d.Get("instance_array_id") != nil {
		ia.InstanceArrayID = d.Get("instance_array_id").(int)
	}
	ia.InstanceArrayLabel = d.Get("instance_array_label").(string)
	ia.InstanceArrayInstanceCount = d.Get("instance_array_instance_count").(int)

	//ia.InstanceArraySubdomain = d.Get("instance_array_subdomain").(string)

	ia.InstanceArrayBootMethod = d.Get("instance_array_boot_method").(string)
	ia.InstanceArrayRAMGbytes = d.Get("instance_array_ram_gbytes").(int)
	ia.InstanceArrayProcessorCount = d.Get("instance_array_processor_count").(int)
	ia.InstanceArrayProcessorCoreMHZ = d.Get("instance_array_processor_core_mhz").(int)
	ia.InstanceArrayProcessorCoreCount = d.Get("instance_array_processor_core_count").(int)
	ia.InstanceArrayDiskCount = d.Get("instance_array_disk_count").(int)
	ia.InstanceArrayDiskSizeMBytes = d.Get("instance_array_disk_size_mbytes").(int)
	ia.VolumeTemplateID = d.Get("volume_template_id").(int)
	ia.InstanceArrayAdditionalWanIPv4JSON = d.Get("instance_array_additional_wan_ipv4_json").(string)

	ia.InstanceArrayFirewallManaged = d.Get("instance_array_firewall_managed").(bool)

	if d.Get("firewall_rule") != nil {
		fwRulesSet := d.Get("firewall_rule").(*schema.Set)
		fwRules := []mc.FirewallRule{}

		for _, fwMap := range fwRulesSet.List() {
			fwRules = append(fwRules, expandFirewallRule(fwMap.(map[string]interface{})))
		}

		ia.InstanceArrayFirewallRules = fwRules
	}

	if d.Get("instance_array_custom_variables") != nil {
		iacv := make(map[string]string)

		for k, v := range d.Get("instance_array_custom_variables").(map[string]interface{}) {
			iacv[k] = v.(string)
		}

		ia.InstanceArrayCustomVariables = iacv
	}

	return ia
}

func flattenFirewallRule(fw mc.FirewallRule) map[string]interface{} {
	var d = make(map[string]interface{})

	d["firewall_rule_description"] = fw.FirewallRuleDescription
	d["firewall_rule_port_range_start"] = fw.FirewallRulePortRangeStart
	d["firewall_rule_port_range_end"] = fw.FirewallRulePortRangeEnd
	d["firewall_rule_source_ip_address_range_start"] = fw.FirewallRuleSourceIPAddressRangeStart
	d["firewall_rule_source_ip_address_range_end"] = fw.FirewallRuleSourceIPAddressRangeEnd
	d["firewall_rule_destination_ip_address_range_start"] = fw.FirewallRuleDestinationIPAddressRangeStart
	d["firewall_rule_destination_ip_address_range_end"] = fw.FirewallRuleDestinationIPAddressRangeEnd
	d["firewall_rule_protocol"] = fw.FirewallRuleProtocol
	d["firewall_rule_ip_address_type"] = fw.FirewallRuleIPAddressType
	d["firewall_rule_enabled"] = fw.FirewallRuleEnabled

	return d
}

func expandFirewallRule(d map[string]interface{}) mc.FirewallRule {
	var fw mc.FirewallRule

	fw.FirewallRuleDescription = d["firewall_rule_description"].(string)
	fw.FirewallRulePortRangeStart = d["firewall_rule_port_range_start"].(int)
	fw.FirewallRulePortRangeEnd = d["firewall_rule_port_range_end"].(int)
	fw.FirewallRuleSourceIPAddressRangeStart = d["firewall_rule_source_ip_address_range_start"].(string)
	fw.FirewallRuleSourceIPAddressRangeEnd = d["firewall_rule_source_ip_address_range_end"].(string)
	fw.FirewallRuleDestinationIPAddressRangeStart = d["firewall_rule_destination_ip_address_range_start"].(string)
	fw.FirewallRuleDestinationIPAddressRangeEnd = d["firewall_rule_destination_ip_address_range_end"].(string)
	fw.FirewallRuleProtocol = d["firewall_rule_protocol"].(string)
	fw.FirewallRuleIPAddressType = d["firewall_rule_ip_address_type"].(string)
	fw.FirewallRuleEnabled = d["firewall_rule_enabled"].(bool)

	return fw
}

func flattenInstanceArrayInterface(i mc.InstanceArrayInterface) map[string]interface{} {
	var d = make(map[string]interface{})

	d["instance_array_interface_id"] = i.InstanceArrayInterfaceID
	d["interface_index"] = i.InstanceArrayInterfaceIndex
	d["network_id"] = i.NetworkID

	return d
}

func expandInstanceArrayInterface(d map[string]interface{}) mc.InstanceArrayInterface {

	var i mc.InstanceArrayInterface

	if d["instance_array_interface_id"] != nil {
		i.InstanceArrayInterfaceID = d["instance_array_interface_id"].(int)
	}

	i.InstanceArrayInterfaceIndex = d["interface_index"].(int)
	i.NetworkID = d["network_id"].(int)

	return i
}

func flattenInstancesCustomVariables(retInstances *map[string]mc.Instance) []interface{} {

	instanceMap := make(map[int]mc.Instance, len(*retInstances))
	keys := []int{}
	instances := []mc.Instance{}

	for _, v := range *retInstances {
		instanceMap[v.InstanceID] = v
		keys = append(keys, v.InstanceID)
	}

	sort.Ints(keys)

	for _, id := range keys {
		instances = append(instances, instanceMap[id])
	}

	customVars := []interface{}{}

	for index, instance := range instances {
		i := make(map[string]interface{})
		cv := make(map[string]interface{})
		i["instance_index"] = index
		switch instance.InstanceCustomVariables.(type) {
		//todo: add nil
		case []interface{}:
			cv = make(map[string]interface{})
		default:
			for k, v := range instance.InstanceCustomVariables.(map[string]interface{}) {
				cv[k] = v.(string)
			}
		}
		i["custom_variables"] = cv
		if len(cv) > 0 {
			customVars = append(customVars, i)
		}
	}

	return customVars
}

//* sets the custom variables on the instances object. Used by the Upgrade function
//TODO: convert tot an actual expand function that doesn't use the client to set them to make it easier to test
func updateInstancesCustomVariables(cvList []interface{}, instanceArrayID int, client *mc.Client) diag.Diagnostics {

	var diags diag.Diagnostics
	instanceList, err := client.InstanceArrayInstances(instanceArrayID)
	if err != nil {
		return diag.FromErr(err)
	}
	//TODO: flatten instances
	instanceMap := make(map[int]mc.Instance, len(*instanceList))
	nInstances := len(*instanceList)
	keys := []int{}
	instances := []mc.Instance{}

	for _, v := range *instanceList {
		instanceMap[v.InstanceID] = v
		keys = append(keys, v.InstanceID)
	}

	sort.Ints(keys)

	for _, id := range keys {
		instances = append(instances, instanceMap[id])
	}

	currentCVLabelList := make(map[string]int, len(*instanceList))

	for _, icvIntf := range cvList {
		icv := icvIntf.(map[string]interface{})
		cvIntf := icv["custom_variables"].(map[string]interface{})
		instance_custom_variables := make(map[string]string)
		for k, v := range cvIntf {
			instance_custom_variables[k] = v.(string)
		}
		instance_index := icv["instance_index"].(int)
		if instance_index < nInstances {
			instance := instances[instance_index]
			currentCVLabelList[instance.InstanceLabel] = instance.InstanceID
			instance.InstanceOperation.InstanceCustomVariables = instance_custom_variables
			_, err := client.InstanceEdit(instance.InstanceID, instance.InstanceOperation)
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}

	for _, instance := range *instanceList {
		if _, ok := currentCVLabelList[instance.InstanceLabel]; !ok {
			instance.InstanceOperation.InstanceCustomVariables = make(map[string]string)
			_, err := client.InstanceEdit(instance.InstanceID, instance.InstanceOperation)
			if err != nil {
				return diag.FromErr(err)
			}
		}
	}
	return diags
}

func copyInstanceArrayToOperation(ia mc.InstanceArray, iao *mc.InstanceArrayOperation) {

	iao.InstanceArrayID = ia.InstanceArrayID
	iao.InstanceArrayLabel = ia.InstanceArrayLabel
	iao.InstanceArrayBootMethod = ia.InstanceArrayBootMethod
	iao.InstanceArrayInstanceCount = ia.InstanceArrayInstanceCount
	iao.InstanceArrayRAMGbytes = ia.InstanceArrayRAMGbytes
	iao.InstanceArrayProcessorCount = ia.InstanceArrayProcessorCount
	iao.InstanceArrayProcessorCoreMHZ = ia.InstanceArrayProcessorCoreMHZ
	iao.InstanceArrayDiskCount = ia.InstanceArrayDiskCount
	iao.InstanceArrayDiskSizeMBytes = ia.InstanceArrayDiskSizeMBytes
	iao.InstanceArrayDiskTypes = ia.InstanceArrayDiskTypes
	iao.ClusterID = ia.ClusterID
	iao.InstanceArrayFirewallManaged = ia.InstanceArrayFirewallManaged
	iao.InstanceArrayFirewallRules = ia.InstanceArrayFirewallRules
	iao.VolumeTemplateID = ia.VolumeTemplateID
	iao.InstanceArrayAdditionalWanIPv4JSON = ia.InstanceArrayAdditionalWanIPv4JSON
	iao.InstanceArrayCustomVariables = ia.InstanceArrayCustomVariables
}

func copyInstanceArrayInterfaceToOperation(i mc.InstanceArrayInterface, io *mc.InstanceArrayInterfaceOperation) {
	io.InstanceArrayInterfaceLAGGIndexes = i.InstanceArrayInterfaceLAGGIndexes
	io.InstanceArrayInterfaceIndex = i.InstanceArrayInterfaceIndex
	io.NetworkID = i.NetworkID
}
