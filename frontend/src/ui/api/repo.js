const list = (payload) => ({
  method: 'AllVolumesAndClones'
})

const create = (payload) => ({
  method: 'Create',
  params: payload
})

const RepoApi = {
  list,
  create
}

export default RepoApi