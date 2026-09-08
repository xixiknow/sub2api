import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import videoCatalog from './videoCatalog'
import videoStudio from './videoStudio'
import videoWorkbench from './videoWorkbench'
import admin from './admin'
import misc from './misc'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  ...videoCatalog,
  ...videoStudio,
  ...videoWorkbench,
  admin,
  ...misc,
}
