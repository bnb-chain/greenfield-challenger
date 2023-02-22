package dao

type DaoManager struct {
	VoteDao  *VoteDao
	EventDao *EventDao
}

func NewDaoManager(eventDao *EventDao, voteDao *VoteDao) *DaoManager {
	return &DaoManager{
		EventDao: eventDao,
		VoteDao:  voteDao,
	}
}
