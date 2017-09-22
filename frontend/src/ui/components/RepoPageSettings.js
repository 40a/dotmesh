import React, { Component, PropTypes } from 'react'

import RouteFactory from 'template-ui/lib/plugins/router/Route'
import { Grid, Row, Col } from 'react-flexbox-grid'
import ListMenu from 'template-ui/lib/components/ListMenu'

import config from '../config'
import * as selectors from '../selectors'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

import RepoPageSettingsCollaborators from './RepoPageSettingsCollaborators'
import RepoPageSettingsAccess from './RepoPageSettingsAccess'

const Route = RouteFactory(config.basepath)

class RepoPageSettings extends Component {

  processMenu(item) {
    if(item.id == this.props.settingsSection) {
      item.theme = {
        itemText: theme.activeMenuItem
      }
    }
    return item
  }

  getCollaborators() {
    return (
      <RepoPageSettingsCollaborators {...this.props} />
    )
  }

  getAccess() {
    return (
      <RepoPageSettingsAccess {...this.props} />
    )
  }

  render() {
    const repo = this.props.repo || {}
    return (
      <div className={ theme.settingsContainer }>
        <div className={ theme.settingsMenu }>
          <ListMenu
            options={ this.props.settingsMenuOptions }
            onClick={ this.props.onSettingsMenuClick }
            process={ this.processMenu.bind(this) }
          />
        </div>
        <div className={ theme.settingsPage }>
          <Route route='/:namespace/:name/settings' exact>
            {this.getCollaborators()}
          </Route>
          <Route route='/:namespace/:name/settings/collaborators' exact>
            {this.getCollaborators()}
          </Route>
          <Route route='/:namespace/:name/settings/access' exact>
            {this.getAccess()}
          </Route>
        </div>
      </div>
    )
  }
}

export default RepoPageSettings