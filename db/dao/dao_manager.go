package dao

type DaoManager struct {
	*BlockDao
	*EventDao
	*VoteDao
}

func NewDaoManager(blockDao *BlockDao, eventDao *EventDao, voteDao *VoteDao) *DaoManager {
	return &DaoManager{
		BlockDao: blockDao,
		EventDao: eventDao,
		VoteDao:  voteDao,
	}
}
