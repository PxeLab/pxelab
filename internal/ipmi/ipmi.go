package ipmi

import (
	"context"
	"fmt"
	"time"

	"github.com/bougou/go-ipmi"
	"github.com/pxego/pxego/internal/models"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) connect(host *models.Host) (*ipmi.Client, error) {
	client, err := ipmi.NewClient(host.BMCAddr, 623, host.BMCUser, host.BMCPass)
	if err != nil {
		return nil, fmt.Errorf("IPMI 创建客户端失败: %w", err)
	}
	client.WithTimeout(5 * time.Second)
	if err := client.Connect(context.Background()); err != nil {
		return nil, fmt.Errorf("IPMI 连接失败: %w", err)
	}
	return client, nil
}

func (c *Client) PowerOn(host *models.Host) error {
	client, err := c.connect(host)
	if err != nil {
		return err
	}
	defer client.Close(context.Background())
	_, err = client.ChassisControl(context.Background(), ipmi.ChassisControlPowerUp)
	return err
}

func (c *Client) PowerOff(host *models.Host) error {
	client, err := c.connect(host)
	if err != nil {
		return err
	}
	defer client.Close(context.Background())
	_, err = client.ChassisControl(context.Background(), ipmi.ChassisControlPowerDown)
	return err
}

func (c *Client) PowerCycle(host *models.Host) error {
	client, err := c.connect(host)
	if err != nil {
		return err
	}
	defer client.Close(context.Background())
	_, err = client.ChassisControl(context.Background(), ipmi.ChassisControlPowerCycle)
	return err
}

func (c *Client) PowerStatus(host *models.Host) (string, error) {
	client, err := c.connect(host)
	if err != nil {
		return "", err
	}
	defer client.Close(context.Background())
	status, err := client.GetChassisStatus(context.Background())
	if err != nil {
		return "", err
	}
	if status.PowerIsOn {
		return "on", nil
	}
	return "off", nil
}
