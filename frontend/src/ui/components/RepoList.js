import React, { Component, PropTypes } from 'react'
import RepoListItem from './RepoListItem'
import theme from './theme/repolist.css'

class RepoList extends Component {

  noData() {
    return (
      <div className={ theme.container }>
        No repos - display help page
      </div>
    )
  }

  dataList() {
    const data = this.props.data || []
    return (
      <div className={ theme.container }>
        {
          data.map((repo, i) => {
            return (
              <RepoListItem
                key={ i }
                repo={ repo }
              />
            )
          })
        }
      </div>
    )
  }

  render() {
    const data = this.props.data || []
    return data.length > 0 ? this.dataList() : this.noData()
  }
}

export default RepoList