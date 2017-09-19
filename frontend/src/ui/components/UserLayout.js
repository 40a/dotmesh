import React, { Component, PropTypes } from 'react'
import { Grid, Row, Col } from 'react-flexbox-grid'
import { Card, CardMedia, CardTitle, CardText, CardActions } from 'react-toolbox/lib/card'

import UserImage from './UserImage'

import theme from './theme/userlayout.css'

class Dashboard extends Component {
  render() {
    return (
      <Grid fluid>
        <Row>
          <Col xs={12} sm={3}>
            <UserImage
              user={ this.props.user }
              imageClassName={ theme.avatar }
            />
          </Col>
          <Col xs={12} sm={9}>
            { this.props.children }
          </Col>
        </Row>
      </Grid>
    )
  }
}

export default Dashboard