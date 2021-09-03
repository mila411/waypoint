import MediaObject from 'components/homepage/media-object'
import Section, {
  SectionHeading,
  SectionDescription,
} from 'components/homepage/section'
import s from './style.module.css'

export default function SectionExtendPlugins() {
  return (
    <Section className={s.extendPlugins}>
      <div className={s.content}>
        <SectionHeading>Extend Waypoint with plugins</SectionHeading>
        <SectionDescription>
          Extend workflows via built-in plugins and an extensible interface.
          Supports custom builders, deployment platforms, registries, release
          managers, and more
        </SectionDescription>
        <div className={s.contentBlocks}>
          <MediaObject
            stacked={true}
            icon={require('../icons/box.svg?include')}
            heading="Available Plugins"
            description="View a list of existing HashiCorp maintained plugins"
          />
          <MediaObject
            stacked={true}
            icon={require('../icons/code-union.svg?include')}
            heading="Creating Waypoint Plugins"
            description="Learn to extend Waypoint for your project’s needs"
          />
        </div>
      </div>
    </Section>
  )
}
