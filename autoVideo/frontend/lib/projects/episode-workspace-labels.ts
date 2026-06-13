export function getEpisodeWorkspaceLabels(isSerial: boolean) {
  return {
    storyboardStageLabel: isSerial ? '镜头拆分与首帧' : '镜头拆分与出图',
    storyboardWorkspaceLabel: isSerial ? '镜头工作台' : '分镜工作台',
    storyboardImageLabel: isSerial ? '首帧图片' : '分镜图片',
  }
}
