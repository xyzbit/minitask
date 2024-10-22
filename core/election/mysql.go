package election

import (
	"log"
	"net"
	"time"

	"github.com/xyzbit/minitaskx/pkg"

	"gorm.io/gorm"
)

type mysqlLeaderElector struct {
	endpoint string
	db       *gorm.DB
}

func NewLeaderElector(port string, db *gorm.DB) Interface {
	if db == nil {
		panic("db is nil")
	}

	ip, err := pkg.GlobalUnicastIPString()
	if err != nil {
		panic(err)
	}
	endpoint := net.JoinHostPort(ip, port)
	log.Println("ip:", ip, "port:", port, "endpoint:", endpoint)

	return &mysqlLeaderElector{endpoint: endpoint, db: db}
}

func (m *mysqlLeaderElector) Leader() (*LeaderElection, error) {
	var leader LeaderElectionPO
	err := m.db.Model(&LeaderElectionPO{}).
		Where("anchor = ?", 1).
		First(&leader).Error
	if err != nil {
		return nil, err
	}
	return &LeaderElection{
		Anchor:         leader.Anchor,
		Endpoint:       leader.Endpoint,
		LastSeenActive: leader.LastSeenActive,
		MasterID:       leader.MasterID,
	}, nil
}

func (m *mysqlLeaderElector) AmILeader(leader *LeaderElection) bool {
	if m == nil || leader == nil {
		return false
	}
	return m.endpoint == leader.Endpoint
}

func (m *mysqlLeaderElector) AttemptElection() {
	sql := `
		INSERT IGNORE INTO devops.leader_election (anchor, endpoint, last_seen_active)
		VALUES (1, ?, NOW())
		ON DUPLICATE KEY UPDATE
			endpoint = IF(last_seen_active < NOW() - INTERVAL 15 SECOND, VALUES(endpoint), endpoint),
			last_seen_active = IF(endpoint = VALUES(endpoint), VALUES(last_seen_active), last_seen_active)
	`
	for {
		err := m.db.Exec(sql, m.endpoint).Error
		if err != nil {
			log.Println("attemptElection error:", err)
		}
		time.Sleep(3 * time.Second)
	}
}

type LeaderElectionPO struct {
	Anchor         int
	MasterID       string
	Endpoint       string
	LastSeenActive time.Time
}

func (le *LeaderElectionPO) TableName() string {
	return "leader_election"
}
