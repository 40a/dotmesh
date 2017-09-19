import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'
import { Card, CardMedia, CardTitle, CardText, CardActions } from 'react-toolbox/lib/card'

import VolumeTable from '../containers/VolumeTable'
import UserImage from './UserImage'

class Dashboard extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col xs={12} sm={4}>
            <UserImage user={ this.props.user } />
          </Col>
          <Col xs={12} sm={8}>
            <VolumeTable />
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Dashboard