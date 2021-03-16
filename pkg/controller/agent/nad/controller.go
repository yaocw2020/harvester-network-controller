package nad

import (
	"context"
	"encoding/json"
	"errors"

	nadv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	harvnetwork "github.com/rancher/harvester/pkg/api/network"
	"github.com/vishvananda/netlink"
	"k8s.io/klog"

	"github.com/rancher/harvester-network-controller/pkg/config"
	"github.com/rancher/harvester-network-controller/pkg/generated/controllers/network.harvester.cattle.io/v1alpha1"
	"github.com/rancher/harvester-network-controller/pkg/network/vlan"
)

// Harvester network nad watches network-attachment-definition CR, retrieve network configuration and make it effective.
// For example, the controller get VLAN ID from nad and add it to physical NIC attached with bridge.
const (
	controllerName = "harvester-network-nad-controller"
)

type Handler struct {
	nodeNetworkCache v1alpha1.NodeNetworkCache
}

func Register(ctx context.Context, management *config.Management) error {
	nad := management.CniFactory.K8s().V1().NetworkAttachmentDefinition()
	nns := management.HarvesterNetworkFactory.Network().V1alpha1().NodeNetwork()

	handler := &Handler{
		nodeNetworkCache: nns.Cache(),
	}

	nad.OnChange(ctx, controllerName, handler.OnChange)
	nad.OnRemove(ctx, controllerName, handler.OnRemove)

	return nil
}

func (h Handler) OnChange(key string, nad *nadv1.NetworkAttachmentDefinition) (*nadv1.NetworkAttachmentDefinition, error) {
	if nad == nil || nad.DeletionTimestamp != nil {
		return nil, nil
	}

	if nad.Spec.Config == "" {
		return nad, nil
	}

	klog.Infof("nad configuration %s has been changed: %s", nad.Name, nad.Spec.Config)
	netconf := &harvnetwork.NetConf{}
	if err := json.Unmarshal([]byte(nad.Spec.Config), netconf); err != nil {
		return nil, err
	}

	// TODO delete previous vlan id when update nad

	v, err := vlan.GetVlan()
	if err != nil && !errors.As(err, &netlink.LinkNotFoundError{}) && !errors.As(err, &vlan.SlaveNotFound{}) {
		return nil, err
	}
	if errors.As(err, &netlink.LinkNotFoundError{}) || errors.As(err, &vlan.SlaveNotFound{}) {
		klog.Infof("ignore link not found error, details: %+v", err)
		return nad, nil
	}

	if err := v.AddLocalArea(netconf.Vlan); err != nil {
		return nil, err
	}

	return nad, nil
}

func (h Handler) OnRemove(key string, nad *nadv1.NetworkAttachmentDefinition) (*nadv1.NetworkAttachmentDefinition, error) {
	if nad == nil {
		return nil, nil
	}

	if nad.Spec.Config == "" {
		return nad, nil
	}

	klog.Infof("nad configuration %s has been deleted.", nad.Name)

	netconf := &harvnetwork.NetConf{}
	if err := json.Unmarshal([]byte(nad.Spec.Config), netconf); err != nil {
		return nil, err
	}

	v, err := vlan.GetVlan()
	if err != nil && !errors.As(err, &netlink.LinkNotFoundError{}) && !errors.As(err, &vlan.SlaveNotFound{}) {
		return nil, err
	}
	if errors.As(err, &netlink.LinkNotFoundError{}) || errors.As(err, &vlan.SlaveNotFound{}) {
		klog.Infof("ignore link not found error, details: %+v", err)
		return nad, nil
	}

	if err := v.RemoveLocalArea(netconf.Vlan); err != nil {
		return nil, err
	}

	return nad, nil
}
