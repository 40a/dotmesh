import React, { Component, PropTypes } from 'react'
import { Table, TableHead, TableRow, TableCell } from 'react-toolbox/lib/table'

class VolumeTable extends Component {
  render() {
    const data = this.props.data || []
    return (
      <Table selectable={ false } style={{ marginTop: 10 }}>
        <TableHead>
          <TableCell>Name</TableCell>
          <TableCell>Size</TableCell>
          <TableCell>Master</TableCell>
        </TableHead>
        {data.map((item, idx) => {
          const volume = item.TopLevelVolume || {}
          return (
            <TableRow key={idx}>
              <TableCell>{volume.Name || ''}</TableCell>
              <TableCell>{volume.SizeBytes || ''}</TableCell>
              <TableCell>{volume.Master || ''}</TableCell>
            </TableRow>
          )
        })}
      </Table>
    )
  }
}

export default VolumeTable