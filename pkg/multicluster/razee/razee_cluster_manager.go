// Copyright 2021 IBM Corp.
// SPDX-License-Identifier: Apache-2.0

package razee

import (
	"fmt"
	corev1 "k8s.io/api/core/v1"
	"strconv"
	"strings"

	"emperror.dev/errors"
	"github.com/IBM/satcon-client-go/client"
	"github.com/IBM/satcon-client-go/client/auth/apikey"
	"github.com/IBM/satcon-client-go/client/auth/iam"
	"github.com/IBM/satcon-client-go/client/auth/local"
	"github.com/IBM/satcon-client-go/client/types"
	"github.com/ghodss/yaml"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"fybrik.io/fybrik/manager/apis/app/v1alpha1"
	"fybrik.io/fybrik/pkg/multicluster"
)

const (
	clusterMetadataConfigMapSL = "/api/v1/namespaces/fybrik-system/configmaps/cluster-metadata"
	endPointURL                = "https://config.satellite.cloud.ibm.com/graphql"
	bluePrintSelfLink          = "/apis/app.fybrik.io/v1alpha1/namespaces/%s/blueprints/%s"
	channelNameTemplate        = "fybrik.io-%s-%s"
	groupNameTemplate          = "fybrik-%s"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	_ = v1alpha1.AddToScheme(scheme)
}

type razeeClusterManager struct {
	orgID        string
	clusterGroup string
	con          client.SatCon
	log          logr.Logger
}

func (r *razeeClusterManager) GetClusters() ([]multicluster.Cluster, error) {
	var clusters []multicluster.Cluster
	var razeeClusters []types.Cluster
	var err error
	if r.clusterGroup != "" {
		r.log.Info("Using clusterGroup to fetch cluster info", "clusterGroup", r.clusterGroup)
		group, err := r.con.Groups.GroupByName(r.orgID, r.clusterGroup)
		if err != nil {
			return nil, err
		}
		razeeClusters = group.Clusters
	} else {
		r.log.Info("Using all clusters in organization as reference clusters.")
		razeeClusters, err = r.con.Clusters.ClustersByOrgID(r.orgID)
		if err != nil {
			return nil, err
		}
	}

	for _, c := range razeeClusters {
		resourceContent, err := r.con.Resources.ResourceContent(r.orgID, c.ClusterID, clusterMetadataConfigMapSL)
		if err != nil {
			r.log.Error(err, "Could not fetch cluster information", "cluster", c.Name)
			return nil, err
		}
		// If no content for the resource was found the cluster is not part of Fybrik or not installed
		// correctly. Fybrik should ignore those clusters and continue.
		if resourceContent == nil {
			r.log.Info("Resource content returned is nil! Skipping cluster", "cluster", c.Name)
			continue
		}
		scheme := runtime.NewScheme()
		cmcm := corev1.ConfigMap{}
		err = multicluster.Decode(resourceContent.Content, scheme, &cmcm)
		if err != nil {
			return nil, err
		}
		cluster := multicluster.CreateCluster(cmcm)
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func createBluePrintSelfLink(namespace string, name string) string {
	return fmt.Sprintf(bluePrintSelfLink, namespace, name)
}

func (r *razeeClusterManager) GetBlueprint(clusterName string, namespace string, name string) (*v1alpha1.Blueprint, error) {
	selfLink := createBluePrintSelfLink(namespace, name)
	cluster, err := r.con.Clusters.ClusterByName(r.orgID, clusterName)
	if err != nil {
		return nil, err
	}
	jsonData, err := r.con.Resources.ResourceContent(r.orgID, cluster.ClusterID, selfLink)
	if err != nil {
		r.log.Error(err, "Error while fetching resource content of blueprint", "cluster", clusterName, "name", name)
		return nil, err
	}
	if jsonData == nil {
		r.log.Info("Could not get any resource data", "cluster", cluster, "namespace", namespace, "name", name)
		return nil, nil
	}
	r.log.V(2).Info("Blueprint data: '" + jsonData.Content + "'")

	if jsonData.Content == "" {
		r.log.Info("Retrieved empty data for ", "cluster", cluster, "namespace", namespace, "name", name)
		return nil, nil
	}

	_ = v1alpha1.AddToScheme(scheme)
	blueprint := v1alpha1.Blueprint{}
	err = multicluster.Decode(jsonData.Content, scheme, &blueprint)
	if blueprint.Namespace == "" {
		r.log.Info("Retrieved an empty blueprint for ", "cluster", cluster, "namespace", namespace, "name", name)
		return nil, nil
	}
	return &blueprint, err
}

func getGroupName(cluster string) string {
	return fmt.Sprintf(groupNameTemplate, cluster)
}

type Collection struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Items           []metav1.Object `json:"items" protobuf:"bytes,2,rep,name=items"`
}

func (r *razeeClusterManager) CreateBlueprint(cluster string, blueprint *v1alpha1.Blueprint) error {
	groupName := getGroupName(cluster)
	channelName := channelName(cluster, blueprint.Name)
	version := "0"

	content, err := yaml.Marshal(blueprint)
	if err != nil {
		return err
	}

	r.log.Info("Blueprint content to create: " + string(content))
	rCluster, err := r.con.Clusters.ClusterByName(r.orgID, cluster)
	if err != nil {
		return errors.Wrap(err, "error while fetching cluster by name")
	}
	if rCluster == nil {
		return fmt.Errorf("no cluster found for orgID %v and cluster name %v", r.orgID, cluster)
	}

	// check group exists
	group, err := r.con.Groups.GroupByName(r.orgID, groupName)
	if err != nil {
		if err.Error() == "Cannot destructure property 'req_id' of 'context' as it is undefined." {
			r.log.Info("Group does not exist. Creating group.")
		} else {
			r.log.Error(err, "Error while fetching group by name", "group", groupName)
			return err
		}
	}
	var groupUUID string
	if group == nil {
		addGroup, err := r.con.Groups.AddGroup(r.orgID, groupName)
		if err != nil {
			return err
		}
		groupUUID = addGroup.UUID
	} else {
		groupUUID = group.UUID
	}

	_, err = r.con.Groups.GroupClusters(r.orgID, groupUUID, []string{rCluster.ClusterID})
	if err != nil {
		r.log.Error(err, "Error while creating group", "group", groupName, "cluster", rCluster, "groupUUID", groupUUID)
		return err
	}

	// Check if channel exists
	existingChannel, err := r.con.Channels.ChannelByName(r.orgID, channelName)
	if err != nil {
		if !strings.HasPrefix(err.Error(), "Query channelByName error.") {
			return err
		}
	}
	if existingChannel != nil {
		// Channel already exists. Update channel instead of creating
		r.log.Info("Channel already exists! Updating channel version...", "existingChannel", existingChannel)
		return r.UpdateBlueprint(cluster, blueprint)
	}

	// create channel
	channel, err := r.con.Channels.AddChannel(r.orgID, channelName)
	if err != nil {
		return err
	}

	// create channel version
	channelVersion, err := r.con.Versions.AddChannelVersion(r.orgID, channel.UUID, version, content, "")
	if err != nil {
		// Remove channel if channelVersion could not be created
		removeChannel, channelRemoveErr := r.con.Channels.RemoveChannel(r.orgID, channel.UUID)
		if channelRemoveErr != nil {
			r.log.Error(channelRemoveErr, "Unable to remove channel after error")
		} else if removeChannel.Success {
			r.log.Info("Rolled back channel version after error")
		}

		return err
	}

	// create subscription
	_, err = r.con.Subscriptions.AddSubscription(r.orgID, channelName, channel.UUID, channelVersion.VersionUUID, []string{groupName})
	if err != nil {
		// Remove channelVersion and channel if the subscription could not be created
		removeChannelVersion, versionRemoveErr := r.con.Versions.RemoveChannelVersion(r.orgID, channelVersion.VersionUUID)
		if versionRemoveErr != nil {
			r.log.Error(versionRemoveErr, "Unable to remove channel version after error")
		} else if removeChannelVersion.Success {
			r.log.Info("Rolled back channel version after error")
		}
		removeChannel, channelRemoveErr := r.con.Channels.RemoveChannel(r.orgID, channel.UUID)
		if channelRemoveErr != nil {
			r.log.Error(channelRemoveErr, "Unable to remove channel after error")
		} else if removeChannel.Success {
			r.log.Info("Rolled back channel after error")
		}
		return err
	}

	r.log.Info("Successfully created subscription!")
	return nil
}

func (r *razeeClusterManager) UpdateBlueprint(cluster string, blueprint *v1alpha1.Blueprint) error {
	channelName := channelName(cluster, blueprint.Name)

	content, err := yaml.Marshal(blueprint)
	if err != nil {
		return err
	}
	r.log.Info("Blueprint content to update: " + string(content))

	max := 0
	channelInfo, err := r.con.Channels.ChannelByName(r.orgID, channelName)
	if err != nil {
		return fmt.Errorf("cannot fetch channel info for channel '%s'", channelName)
	}
	for _, version := range channelInfo.Versions {
		v, err := strconv.Atoi(version.Name)
		if err != nil {
			return fmt.Errorf("cannot parse version name %s", version.Name)
		} else if max < v {
			max = v
		}
	}

	nextVersion := strconv.Itoa(max + 1)

	// There is only one subscription per channel in our use case
	if len(channelInfo.Subscriptions) != 1 {
		return errors.New("found more or less than one subscription")
	}
	subscriptionUUID := channelInfo.Subscriptions[0].UUID

	r.log.V(1).Info("Creating new channel version", "nextVersion", nextVersion, "subscriptionUUID", subscriptionUUID, "channelUuid", channelInfo.UUID)

	// create channel version
	channelVersion, err := r.con.Versions.AddChannelVersion(r.orgID, channelInfo.UUID, nextVersion, content, "")
	if err != nil {
		r.log.Error(err, "er")
		return err
	}

	r.log.V(2).Info("Updating subscription...")

	// update subscription
	_, err = r.con.Subscriptions.SetSubscription(r.orgID, subscriptionUUID, channelVersion.VersionUUID)
	if err != nil {
		return err
	}

	r.log.Info("Subscription successfully updated!")

	return nil
}

func (r *razeeClusterManager) DeleteBlueprint(cluster string, namespace string, name string) error {
	channelName := channelName(cluster, name)
	channel, err := r.con.Channels.ChannelByName(r.orgID, channelName)
	if err != nil {
		return err
	}
	for _, s := range channel.Subscriptions {
		subscription, err := r.con.Subscriptions.RemoveSubscription(r.orgID, s.UUID)
		if err != nil {
			return err
		}
		if subscription.Success {
			r.log.Info("Successfully deleted subscription " + subscription.UUID)
		}
	}
	for _, v := range channel.Versions {
		version, err := r.con.Versions.RemoveChannelVersion(r.orgID, v.UUID)
		if err != nil {
			return err
		}
		if version.Success {
			r.log.Info("Successfully deleted version " + version.UUID)
		}
	}

	removeChannel, err := r.con.Channels.RemoveChannel(r.orgID, channel.UUID)
	if err != nil {
		return err
	}
	if removeChannel.Success {
		r.log.Info("Successfully deleted channel " + removeChannel.UUID)
	}
	return nil
}

// The channel name should be per cluster and plotter, so it cannot be based on
// the namespace that is random for every blueprint
func channelName(cluster string, name string) string {
	return fmt.Sprintf(channelNameTemplate, cluster, name)
}

// NewRazeeLocalClusterManager creates an instance of Razee based ClusterManager with userName/password authentication
func NewRazeeLocalClusterManager(url string, login string, password string, clusterGroup string) (multicluster.ClusterManager, error) {
	localAuth, err := local.NewClient(url, login, password)
	if err != nil {
		return nil, err
	}
	con, _ := client.New(url, localAuth)
	logger := ctrl.Log.WithName("RazeeManager")
	me, err := con.Users.Me()
	if err != nil {
		return nil, err
	}

	if me == nil {
		return nil, errors.New("could not retrieve login information of Razee")
	}

	logger.Info("Initializing Razee local", "orgId", me.OrgId, "clusterGroup", clusterGroup)

	return &razeeClusterManager{
		orgID:        me.OrgId,
		clusterGroup: clusterGroup,
		con:          con,
		log:          logger,
	}, nil
}

// NewRazeeOAuthClusterManager creates an instance of Razee based ClusterManager with OAuth authentication
func NewRazeeOAuthClusterManager(url string, apiKey string, clusterGroup string) (multicluster.ClusterManager, error) {
	auth, err := apikey.NewClient(apiKey)
	if err != nil {
		return nil, err
	}
	con, _ := client.New(url, auth)
	logger := ctrl.Log.WithName("RazeeManager")
	me, err := con.Users.Me()
	if err != nil {
		return nil, err
	}

	if me == nil {
		return nil, errors.New("could not retrieve login information of Razee")
	}

	logger.Info("Initializing Razee using oauth", "orgId", me.OrgId, "clusterGroup", clusterGroup)

	return &razeeClusterManager{
		orgID:        me.OrgId,
		clusterGroup: clusterGroup,
		con:          con,
		log:          logger,
	}, nil
}

// NewSatConfClusterManager creates an instance of Razee based ClusterManager with Satellite authentication
func NewSatConfClusterManager(apikey string, clusterGroup string) (multicluster.ClusterManager, error) {
	iamClient, err := iam.NewIAMClient(apikey, "")
	if err != nil {
		return nil, err
	}
	if iamClient == nil {
		return nil, errors.New("the IAMClient returned nil for IBM Cloud Satellite Config")
	}
	con, err := client.New(endPointURL, iamClient.Client)
	if err != nil {
		return nil, err
	}

	me, err := con.Users.Me()
	if err != nil {
		return nil, err
	}

	if me == nil {
		return nil, errors.New("could not retrieve login information of Razee")
	}

	logger := ctrl.Log.WithName("RazeeManager")

	logger.Info("Initializing Razee with IBM Satellite Config", "orgId", me.OrgId, "clusterGroup", clusterGroup)

	return &razeeClusterManager{
		orgID:        me.OrgId,
		clusterGroup: clusterGroup,
		con:          con,
		log:          logger,
	}, nil
}
