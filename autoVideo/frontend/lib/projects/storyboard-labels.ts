export function getStoryboardTabLabels(isSerial: boolean) {
  return {
    storyboardItemLabel: isSerial ? '镜头' : '分镜',
    extractStoryboardLabel: isSerial ? '提取当前集镜头并分组' : '提取当前集镜头',
    storyboardGenerateLabel: isSerial ? '首帧生成' : '分镜图片生成',
    storyboardImageLabel: isSerial ? '首帧图片' : '分镜图片',
    storyboardVideoLabel: isSerial ? '串行视频' : '视频',
  }
}
