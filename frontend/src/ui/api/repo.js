const list = (payload) => ({
  method: 'AllVolumesAndClones'
})

// payload is {Namespace,Name}
const create = (payload) => ({
  method: 'Create',
  params: payload
})

// payload is branch id
const commits = (payload) => ({
  method: 'SnapshotsById',
  params: [payload]
})

const RepoApi = {
  list,
  create,
  commits
}

export default RepoApi