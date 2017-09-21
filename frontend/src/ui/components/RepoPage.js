import React, { Component, PropTypes } from 'react'

import { Tab, Tabs } from 'react-toolbox'
import { Grid, Row, Col } from 'react-flexbox-grid'

import * as selectors from '../selectors'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

import RepoPageData from './RepoPageData'
import RepoPageSettings from './RepoPageSettings'

class RepoPage extends Component {

  repoName() {
    const repo = this.props.repo || {}
    return (
      <div className={ theme.largeTitle }>
        <div className={ [theme.repoName, colors.bluelink, theme.link].join(' ') } onClick={ () => this.props.clickNamespace(selectors.repo.namespace(repo)) }>
          { this.props.urlName.Namespace }
        </div>
        &nbsp;/&nbsp;
        <div className={ theme.repoName }>
          { this.props.urlName.Name }
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
    )
  }

  tabs() {
    let activeIndex = 0
    const path = this.props.urlName.Section || ''
    if(path.indexOf('settings') == 0) {
      activeIndex = 1
    }
    
    return (
      <Tabs index={activeIndex}>
        <Tab label='Data' onClick={ () => this.props.clickTab(this.props.repo) }>
          <RepoPageData {...this.props} />
        </Tab>
        <Tab label='Settings' onClick={ () => this.props.clickTab(this.props.repo, 'settings') }>
          <RepoPageSettings {...this.props} />
        </Tab>
      </Tabs>
    )
  }

  render() {
    const repo = this.props.repo || {}
    return (
      <Grid fluid>
        <Row>
          <Col xs={12} sm={10} md={8} smOffset={1} mdOffset={2}>
            { this.repoName() }
            { this.tabs() }
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default RepoPage