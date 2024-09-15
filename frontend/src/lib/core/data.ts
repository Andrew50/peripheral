
export let setups<
privateRequest<Setup[]>('getSetups', {})
  .then((v: Setup[]) => {
    setups.set(v);
  })
  .catch((error) => {
    console.error('Error fetching setups:', error);
  });
