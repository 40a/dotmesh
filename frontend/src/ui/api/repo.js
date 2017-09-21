const list = (payload) => ({
  method: 'AllVolumesAndClones'
})

// payload is {Namespace,Name}
const create = (payload) => ({
  method: 'Create',
  params: payload
})

// payload is branch id
const loadCommits = (payload) => ({
  method: 'SnapshotsById',
  params: [payload]
})

const RepoApi = {
  list,
  create,
  loadCommits
}

export default RepoApi