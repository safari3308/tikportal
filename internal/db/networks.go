package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/canhnm/tikportal/internal/ipam"
)

type WGNetwork struct {
	ID                  int64     `json:"id"`
	Name                string    `json:"name"`
	Description         string    `json:"description"`
	ROSInterface        string    `json:"ros_interface"`
	ServerPublicKey     string    `json:"server_public_key"`
	SubnetCIDR          string    `json:"subnet_cidr"`
	ServerIP            string    `json:"server_ip"`
	EndpointHost        string    `json:"endpoint_host"`
	EndpointPort        int       `json:"endpoint_port"`
	DNSServers          string    `json:"dns_servers"`
	AllowedIPs          string    `json:"allowed_ips"`
	PersistentKeepalive int       `json:"persistent_keepalive"`
	Enabled             bool      `json:"enabled"`
	AllocatedCount      int       `json:"allocated_count"`
	SubnetCapacity      int       `json:"subnet_capacity"`
	CreatedAt           time.Time `json:"created_at"`
}

func (s *Store) ListNetworks(ctx context.Context, onlyEnabled bool) ([]WGNetwork, error) {
	query := `
		SELECT n.id, n.name, n.description, n.ros_interface, n.server_public_key,
		       n.subnet_cidr, n.server_ip, n.endpoint_host, n.endpoint_port,
		       n.dns_servers, n.allowed_ips, n.persistent_keepalive, n.enabled, n.created_at,
		       (SELECT COUNT(*) FROM wg_peers p WHERE p.network_id = n.id) AS allocated_count
		FROM wg_networks n
	`
	if onlyEnabled {
		query += ` WHERE n.enabled = 1 `
	}
	query += ` ORDER BY n.id ASC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []WGNetwork
	for rows.Next() {
		var n WGNetwork
		var enabledInt int
		if err := rows.Scan(
			&n.ID, &n.Name, &n.Description, &n.ROSInterface, &n.ServerPublicKey,
			&n.SubnetCIDR, &n.ServerIP, &n.EndpointHost, &n.EndpointPort,
			&n.DNSServers, &n.AllowedIPs, &n.PersistentKeepalive, &enabledInt, &n.CreatedAt,
			&n.AllocatedCount,
		); err != nil {
			return nil, err
		}
		n.Enabled = enabledInt != 0
		if info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP); err == nil {
			n.SubnetCapacity = info.Capacity
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

func (s *Store) GetNetworkByID(ctx context.Context, id int64) (*WGNetwork, error) {
	var n WGNetwork
	var enabledInt int
	err := s.db.QueryRowContext(ctx, `
		SELECT n.id, n.name, n.description, n.ros_interface, n.server_public_key,
		       n.subnet_cidr, n.server_ip, n.endpoint_host, n.endpoint_port,
		       n.dns_servers, n.allowed_ips, n.persistent_keepalive, n.enabled, n.created_at,
		       (SELECT COUNT(*) FROM wg_peers p WHERE p.network_id = n.id) AS allocated_count
		FROM wg_networks n
		WHERE n.id = ?
	`, id).Scan(
		&n.ID, &n.Name, &n.Description, &n.ROSInterface, &n.ServerPublicKey,
		&n.SubnetCIDR, &n.ServerIP, &n.EndpointHost, &n.EndpointPort,
		&n.DNSServers, &n.AllowedIPs, &n.PersistentKeepalive, &enabledInt, &n.CreatedAt,
		&n.AllocatedCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	n.Enabled = enabledInt != 0
	if info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP); err == nil {
		n.SubnetCapacity = info.Capacity
	}
	return &n, nil
}

func (s *Store) CreateNetwork(ctx context.Context, n *WGNetwork) (*WGNetwork, error) {
	info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP)
	if err != nil {
		return nil, err
	}
	n.SubnetCIDR = info.Prefix.String()
	n.ServerIP = info.GatewayIP.String()
	if n.PersistentKeepalive <= 0 {
		n.PersistentKeepalive = 25
	}
	enabledInt := 1
	if !n.Enabled {
		enabledInt = 0
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO wg_networks (
			name, description, ros_interface, server_public_key,
			subnet_cidr, server_ip, endpoint_host, endpoint_port,
			dns_servers, allowed_ips, persistent_keepalive, enabled
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		n.Name, n.Description, n.ROSInterface, n.ServerPublicKey,
		n.SubnetCIDR, n.ServerIP, n.EndpointHost, n.EndpointPort,
		n.DNSServers, n.AllowedIPs, n.PersistentKeepalive, enabledInt,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetNetworkByID(ctx, id)
}

func (s *Store) UpdateNetwork(ctx context.Context, id int64, n *WGNetwork) (*WGNetwork, error) {
	info, err := ipam.ValidateSubnet(n.SubnetCIDR, n.ServerIP)
	if err != nil {
		return nil, err
	}
	n.SubnetCIDR = info.Prefix.String()
	n.ServerIP = info.GatewayIP.String()
	enabledInt := 0
	if n.Enabled {
		enabledInt = 1
	}

	res, err := s.db.ExecContext(ctx, `
		UPDATE wg_networks SET
			name = ?, description = ?, ros_interface = ?, server_public_key = ?,
			subnet_cidr = ?, server_ip = ?, endpoint_host = ?, endpoint_port = ?,
			dns_servers = ?, allowed_ips = ?, persistent_keepalive = ?, enabled = ?
		WHERE id = ?
	`,
		n.Name, n.Description, n.ROSInterface, n.ServerPublicKey,
		n.SubnetCIDR, n.ServerIP, n.EndpointHost, n.EndpointPort,
		n.DNSServers, n.AllowedIPs, n.PersistentKeepalive, enabledInt,
		id,
	)
	if err != nil {
		return nil, err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return nil, ErrNotFound
	}
	return s.GetNetworkByID(ctx, id)
}

func (s *Store) DeleteNetwork(ctx context.Context, id int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM wg_networks WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrNotFound
	}
	return nil
}
