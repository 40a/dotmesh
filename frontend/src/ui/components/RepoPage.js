import React, { Component, PropTypes } from 'react'

import { Grid, Row, Col } from 'react-flexbox-grid'

import * as selectors from '../selectors'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

class RepoPage extends Component {

  render() {
    const repo = this.props.repo || {}
    return (
      <Grid fluid>
        <Row>
          <Col xs={12} sm={8} smOffset={2}>
            <div className={ theme.largeTitle }>
              <div className={ [theme.repoName, colors.bluelink, theme.link].join(' ') } onClick={ () => this.props.clickNamespace(selectors.repo.namespace(repo)) }>
                { selectors.repo.namespace(repo) }
              </div>
              &nbsp;/&nbsp;
              <div className={ theme.repoName }>
                { selectors.repo.name(repo) }
              </div>
              &nbsp;
              {
                selectors.repo.isPrivate(repo) ? (
                  <StatusChip
                    highlight
                  >
                    Private
                  </StatusChip>
                ) : null
              }
            </div>
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default RepoPage