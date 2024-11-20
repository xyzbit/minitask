package election

import (
	"errors"
	"log"
	"time"

	"github.com/xyzbit/minitaskx/pkg"

	"gorm.io/gorm"
)

type mysqlLeaderElector struct {
	id string
	db *gorm.DB
}

func NewLeaderElector(id string, db *gorm.DB) Interface {
	if db == nil {
		panic("db is nil")
	}

	ip, err := pkg.GlobalUnicastIPString()
	if err != nil {
		panic(err)
	}
	if id == "" {
		id = ip
	}
	log.Printf("new mysqlLeaderElector id:%s ip:%s", id, ip)

	return &mysqlLeaderElector{id: id, db: db}
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
		IP:             leader.IP,
		LastSeenActive: leader.LastSeenActive,
		MasterID:       leader.MasterID,
	}, nil
}

func (m *mysqlLeaderElector) AmILeader(leader *LeaderElection) bool {
	if m == nil || leader == nil {
		return false
	}
	return m.id == leader.MasterID
}

func (m *mysqlLeaderElector) AttemptElection() {
	for {

		m.db.Transaction(func(tx *gorm.DB) error {
			// 1. 查询当前的选举记录
			var leader LeaderElectionPO
			// 当前判断是否是leader直接查询的数据库，如果有缓存选举需要是串行的(加上.Clauses(clause.Locking{Strength: "UPDATE"}))
			err := tx.Where("anchor = ?", 1).First(&leader).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
				return tx.Create(&LeaderElectionPO{
					Anchor:         1,
					MasterID:       m.id,
					LastSeenActive: time.Now(),
				}).Error
			}
			// 查询到记录
			if leader.LastSeenActive.Before(time.Now().Add(-15 * time.Second)) {
				// 超过15秒没更新，可以抢占
				leader.MasterID = m.id
				leader.LastSeenActive = time.Now()
				return tx.Where("anchor = ?", 1).Updates(&leader).Error
			} else if leader.MasterID == m.id {
				// 已经是leader，更新活跃时间
				leader.MasterID = ""
				leader.LastSeenActive = time.Now()
				return tx.Where("anchor = ?", 1).Updates(&leader).Error
			}
			return nil
		})

		time.Sleep(3 * time.Second)
	}
}

type LeaderElectionPO struct {
	Anchor         int
	MasterID       string
	IP             string
	LastSeenActive time.Time
}

func (le *LeaderElectionPO) TableName() string {
	return "leader_election"
}
