import React, { Component, PropTypes } from 'react'

import Navigation from 'react-toolbox/lib/navigation'
import ProgressBar from 'react-toolbox/lib/progress_bar'

import config from '../config'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

import CollaboratorListItem from './CollaboratorListItem'
import SearchBox from './widgets/SearchBox'

class RepoPageSettingsCollaborators extends Component {

  search() {
    return (
      <div className={ theme.searchContainer } id="collaborator-search">
        <SearchBox
          label={ 'Find User...' }
          value={ this.props.addCollaboratorName || ''}
          onChange={ this.props.updateAddCollaboratorName }
        />
      </div>
    )
  }

  dataList() {
    const data = this.props.collaborators || []
    return (
      <div className={ theme.listContainer } id="collaborator-list">
        {
          data.map((collaborator, i) => {
            return (
              <CollaboratorListItem
                key={ i }
                collaborator={ collaborator }
              />
            )
          })
        }
      </div>
    )
  }


  buttons() {
    const addValue = this.props.addCollaboratorName || ''
    const actions = [
      { 
        id: 'add',
        label: 'Add',
        accent: addValue.length > 0,
        raised: addValue.length > 0,
        disabled: this.props.collaboratorFormLoading ? true : false,
        icon: config.icons.add,
        onClick: () => this.props.addCollaboratorClick() }
    ]
    return (
      <div className={ theme.buttonsContainer } id="collaborator-buttons">
        <Navigation type='horizontal' actions={actions} />
      </div>
    )
  }

  optionsBar() {
    return (
      <div className={ theme.optionsContainer }>
        { this.search() }
        { this.buttons() }
      </div>
    )
  }

  render() {
    return (
      <div>
        <div className={ theme.branchContainer }>
          <h2 className={ theme.collaboratorsTitle }>Collaborators</h2>
          {
            this.props.loaded ? (
              <div>
                <div className={ theme.commitSearchContainer }>
                  { this.optionsBar() }
                </div>
                <div>
                  { this.dataList() }
                </div>
              </div>
            ) : (
              <div>
                <ProgressBar type='circular' mode='indeterminate' multicolor />
              </div>
            )
          }
        </div>
      </div>
    )
  }
}

export default RepoPageSettingsCollaborators