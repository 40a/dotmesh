import React, { Component, PropTypes } from 'react'

import { Tab, Tabs } from 'react-toolbox'
import { Grid, Row, Col } from 'react-flexbox-grid'

import * as selectors from '../selectors'

import theme from './theme/repo.css'
import colors from './theme/colors.css'

import StatusChip from './widgets/StatusChip'

import RepoPageData from './RepoPageData'
import RepoPageSettings from './RepoPageSettings'

const SECTION_INDEXES = {
  data: 0,
  settings: 1
}

class RepoPage extends Component {

  repoName() {
    const repo = this.props.repo || {}
    return (
      <div className={ theme.largeTitle }>
        <div className={ [theme.repoName, colors.bluelink, theme.link].join(' ') } onClick={ () => this.props.clickNamespace(this.props.info.Namespace) }>
          { this.props.info.Namespace }
        </div>
        &nbsp;/&nbsp;
        <div className={ theme.repoName }>
          { this.props.info.Name }
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
    return (
      <Tabs index={SECTION_INDEXES[this.props.section]}>
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