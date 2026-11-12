import { Card } from "../Card";
import { Divider } from "../Divider";
import { Icon } from "../Icon";
import { Text } from "../Text";
import { snapshots } from "../test-utils";

describe("Card, Divider and Icon", () => {
  snapshots(() => (
    <Card>
      <Text variant="heading">Week 3, day 2</Text>
      <Divider />
      <Icon name="flame" color="accent" />
      <Icon name="checkmark-circle" size={32} color="success" />
    </Card>
  ));
});
