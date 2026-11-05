import adminAccessKeys from './adminAccessKeys'
import mcp from './mcp'
import landing from './landing'
import common from './common'
import checkin from './checkin'
import contact from './contact'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import qqbot from './qqbot'

export default {
  adminAccessKeys,
  mcp,
  ...landing,
  ...common,
  ...checkin,
  ...contact,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
  ...qqbot,
}
