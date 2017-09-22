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

// payload is {Volume: <ID>, Collaborator: <USERNAME>}
const addCollaborator = (payload) => ({
  method: 'AddCollaborator',
  params: payload
})

const RepoApi = {
  list,
  create,
  loadCommits,
  addCollaborator
}

export default RepoApi